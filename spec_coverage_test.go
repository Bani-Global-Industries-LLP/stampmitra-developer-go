package stampmitra

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

type specOp struct{ Method, Template string }

func (o specOp) key() string { return o.Method + " " + o.Template }

func loadSpecOps(t testing.TB, file string) []specOp {
	t.Helper()
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}
	var ops []specOp
	for p, item := range spec.Paths {
		for m := range item {
			switch m {
			case "get", "post", "put", "patch", "delete", "head", "options":
				ops = append(ops, specOp{strings.ToUpper(m), p})
			}
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].key() < ops[j].key() })
	return ops
}

func templateRegexp(tmpl string) *regexp.Regexp {
	re := regexp.QuoteMeta(tmpl)
	re = regexp.MustCompile(`\\\{[^}]*\\\}`).ReplaceAllString(re, `[^/]+`)
	return regexp.MustCompile("^" + re + "$")
}

// TestSpecCoverage fails when an operation in spec/openapi.json has no SDK method, or an SDK
// method sends a request the spec does not describe. Run locally with:
//
//	go test -run TestSpecCoverage .
func TestSpecCoverage(t *testing.T) {
	ops := loadSpecOps(t, "spec/openapi.json")
	if len(ops) != 31 {
		t.Fatalf("spec has %d operations, expected 31", len(ops))
	}
	cases := operationCases()
	if len(cases) != len(ops) {
		t.Errorf("SDK table has %d cases, spec has %d operations", len(cases), len(ops))
	}

	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/services" {
			writeJSON(w, 200, `[]`)
			return
		}
		writeJSON(w, 200, `{}`)
	})
	hit := map[string]string{}
	for _, tc := range cases {
		before := len(env.rec.all())
		if _, err := tc.Call(context.Background(), env.c); err != nil {
			t.Errorf("%s: %v", tc.Name, err)
			continue
		}
		reqs := env.rec.all()[before:]
		if len(reqs) != 1 {
			t.Errorf("%s: %d requests", tc.Name, len(reqs))
			continue
		}
		got := reqs[0].Method + " " + reqs[0].Path
		if got != tc.Method+" "+tc.Path {
			t.Errorf("%s sent %q, table says %q", tc.Name, got, tc.Method+" "+tc.Path)
		}
		matched := false
		for _, op := range ops {
			if op.Method == reqs[0].Method && templateRegexp(op.Template).MatchString(reqs[0].Path) {
				hit[op.key()] = tc.Name
				matched = true
			}
		}
		if !matched {
			t.Errorf("%s sent %s which is not in the spec", tc.Name, got)
		}
	}
	for _, op := range ops {
		if hit[op.key()] == "" {
			t.Errorf("spec operation %s has no SDK method", op.key())
		}
	}
	if len(hit) != len(ops) {
		t.Errorf("covered %d of %d operations", len(hit), len(ops))
	}
}

// TestSpecCoverageDetectsMissingOperation proves the check can fail: a spec with an extra
// operation must be reported as uncovered.
func TestSpecCoverageDetectsMissingOperation(t *testing.T) {
	dir := t.TempDir()
	raw, _ := os.ReadFile("spec/openapi.json")
	var spec map[string]any
	_ = json.Unmarshal(raw, &spec)
	spec["paths"].(map[string]any)["/v1/brand-new"] = map[string]any{"get": map[string]any{}}
	b, _ := json.Marshal(spec)
	f := filepath.Join(dir, "openapi.json")
	_ = os.WriteFile(f, b, 0o600)
	ops := loadSpecOps(t, f)
	covered := map[string]bool{}
	for _, tc := range operationCases() {
		for _, op := range ops {
			if op.Method == tc.Method && templateRegexp(op.Template).MatchString(tc.Path) {
				covered[op.key()] = true
			}
		}
	}
	if covered["GET /v1/brand-new"] || len(covered) != 31 {
		t.Fatalf("coverage check cannot detect a new operation (covered %d)", len(covered))
	}
}

// TestBundledSpecMatchesUpstream keeps spec/openapi.json identical to the spec repo's copy.
func TestBundledSpecMatchesUpstream(t *testing.T) {
	dir := specDir()
	if dir == "" {
		if os.Getenv("CONFORMANCE_REQUIRED") == "1" {
			t.Fatal("STAMPMITRA_SPEC_DIR is required when CONFORMANCE_REQUIRED=1")
		}
		t.Skip("STAMPMITRA_SPEC_DIR not set and no local stampmitra-developer-openapi checkout")
	}
	upstream, err := os.ReadFile(filepath.Join(dir, "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	local, _ := os.ReadFile("spec/openapi.json")
	if !bytes.Equal(upstream, local) {
		t.Fatal("spec/openapi.json differs from the upstream openapi.json: copy it over and re-run scripts/gen_models.py")
	}
}

func TestPostsCarryIdempotencyKeysExceptReplay(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/services" {
			writeJSON(w, 200, `[]`)
			return
		}
		writeJSON(w, 200, `{}`)
	})
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for _, tc := range operationCases() {
		before := len(env.rec.all())
		if _, err := tc.Call(context.Background(), env.c); err != nil {
			t.Fatal(tc.Name, err)
		}
		r := env.rec.all()[before]
		key := r.Header.Get("Idempotency-Key")
		switch {
		case tc.Method == "POST" && !tc.NoKey:
			if !uuid.MatchString(key) {
				t.Errorf("%s: Idempotency-Key = %q", tc.Name, key)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("%s: Content-Type = %q", tc.Name, r.Header.Get("Content-Type"))
			}
		default:
			if key != "" {
				t.Errorf("%s: unexpected Idempotency-Key", tc.Name)
			}
		}
	}
}
