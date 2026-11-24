package stampmitra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type opFixture struct {
	Operation struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	} `json:"operation"`
	Scenario string `json:"scenario"`
	Request  struct {
		Body json.RawMessage `json:"body"`
	} `json:"request"`
	Response struct {
		Status  int               `json:"status"`
		Headers map[string]string `json:"headers"`
		Body    json.RawMessage   `json:"body"`
	} `json:"response"`
}

func loadFixtureFiles(t *testing.T) []opFixture {
	t.Helper()
	dir := specDir()
	if dir == "" {
		if os.Getenv("CONFORMANCE_REQUIRED") == "1" {
			t.Fatal("STAMPMITRA_SPEC_DIR is required when CONFORMANCE_REQUIRED=1")
		}
		t.Skip("no stampmitra-developer-openapi checkout (set STAMPMITRA_SPEC_DIR)")
	}
	files, _ := filepath.Glob(filepath.Join(dir, "fixtures", "operations", "*.json"))
	if len(files) < 400 {
		t.Fatalf("only %d operation fixtures", len(files))
	}
	var out []opFixture
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var fx opFixture
		if err := json.Unmarshal(raw, &fx); err != nil {
			t.Fatal(f, err)
		}
		out = append(out, fx)
	}
	return out
}

// TestFixtureReplay serves every golden response (success, success-2xx and error-N, 411 files, all
// 62 operations) to the matching SDK method and checks decoding, lossless re-encoding and the
// error mapping. It needs the spec repo checkout; the conformance suite runs the same table
// against the real mock server.
func TestFixtureReplay(t *testing.T) {
	fxs := loadFixtureFiles(t)
	byKey := casesByTemplate(t)
	seenOps := map[string]bool{}
	for _, fx := range fxs {
		fx := fx
		key := fx.Operation.Method + " " + fx.Operation.Path
		tc, ok := byKey[key]
		if !ok {
			t.Errorf("fixture for %s has no SDK method", key)
			continue
		}
		seenOps[key] = true
		t.Run(tc.Name+"/"+fx.Scenario, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range fx.Response.Headers {
					w.Header().Set(k, v)
				}
				w.WriteHeader(fx.Response.Status)
				_, _ = w.Write(fx.Response.Body)
			}))
			defer srv.Close()
			c, _ := NewClient(testKey, WithBaseURL(srv.URL), WithMaxRetries(0))
			res, err := tc.Call(context.Background(), c)
			if fx.Response.Status >= 200 && fx.Response.Status < 300 {
				if err != nil {
					t.Fatal(err)
				}
				out, merr := json.Marshal(res)
				if merr != nil {
					t.Fatal(merr)
				}
				var want, got any
				_ = json.Unmarshal(fx.Response.Body, &want)
				_ = json.Unmarshal(out, &got)
				if !reflect.DeepEqual(prune(want), prune(got)) {
					t.Errorf("re-encoded body differs:\n want %s\n  got %s", fx.Response.Body, out)
				}
				if ex := extraOf(res); len(ex) > 0 {
					t.Errorf("fields not modelled: %v", ex)
				}
				return
			}
			var api *APIError
			if !errors.As(err, &api) || api.Status != fx.Response.Status {
				t.Fatalf("got %v", err)
			}
			var env struct {
				Error struct {
					Code, RequestID string
					Details         struct {
						Errors []FieldIssue
						Wallet string
					}
				} `json:"error"`
			}
			_ = json.Unmarshal(fx.Response.Body, &env)
			if api.Code != env.Error.Code || api.RequestID != env.Error.RequestID {
				t.Errorf("code/requestId = %q/%q, want %q/%q", api.Code, api.RequestID, env.Error.Code, env.Error.RequestID)
			}
			if !reflect.DeepEqual(api.FieldErrors, env.Error.Details.Errors) || api.Wallet != env.Error.Details.Wallet {
				t.Errorf("details: %+v %q, want %+v %q", api.FieldErrors, api.Wallet, env.Error.Details.Errors, env.Error.Details.Wallet)
			}
			if s := sentinelForStatus(fx.Response.Status); s != nil && !errors.Is(err, s) {
				t.Errorf("%d does not match its sentinel", fx.Response.Status)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Error("key leaked")
			}
		})
	}
	if len(seenOps) != 62 {
		t.Errorf("fixtures cover %d of 62 operations", len(seenOps))
	}
}

// TestFixtureRequestBodiesRoundTrip decodes the request body of every order fixture into the SDK
// type and re-encodes it: the string-or-object party union must come back in the form it had.
func TestFixtureRequestBodiesRoundTrip(t *testing.T) {
	n := 0
	for _, fx := range loadFixtureFiles(t) {
		var target any
		switch fx.Operation.Method + " " + fx.Operation.Path {
		case "POST /v1/estamp/orders", "POST /v1/estamp/orders/validate":
			target = &EstampOrderParams{}
		case "POST /v1/estamp/stamps/{stampId}/pan-card":
			target = &EstampPANCardParams{}
		case "POST /v1/stamp-duty-wallet/topup", "POST /v1/service-fee-wallet/topup":
			target = &WalletTopupParams{}
		default:
			continue
		}
		if len(fx.Request.Body) == 0 {
			continue
		}
		n++
		name := fmt.Sprintf("%s %s %s", fx.Operation.Method, fx.Operation.Path, fx.Scenario)
		if err := json.Unmarshal(fx.Request.Body, target); err != nil {
			t.Fatal(name, err)
		}
		out, err := json.Marshal(target)
		if err != nil {
			t.Fatal(name, err)
		}
		var want, got any
		_ = json.Unmarshal(fx.Request.Body, &want)
		_ = json.Unmarshal(out, &got)
		if !reflect.DeepEqual(want, got) {
			t.Errorf("%s: request body changed:\n want %s\n  got %s", name, fx.Request.Body, out)
		}
	}
	if n < 10 {
		t.Fatalf("only %d request bodies checked", n)
	}
}
