package stampmitra

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

type retryScenario struct {
	Name                           string          `json:"name"`
	Method                         string          `json:"method"`
	Path                           string          `json:"path"`
	IdempotencyKey                 json.RawMessage `json:"idempotencyKey"`
	MaxRetries                     *int            `json:"maxRetries"`
	Sequence                       []any           `json:"sequence"`
	ExpectedAttempts               int             `json:"expectedAttempts"`
	SameIdempotencyKeyAcrossAlways bool            `json:"sameIdempotencyKeyAcrossAttempts"`
	Outcome                        any             `json:"outcome"`
	Optional                       bool            `json:"optional"`
	RetrySafe                      bool            `json:"retrySafe"`
	ExpectedDelay                  *float64        `json:"expectedDelayBeforeAttempt2Seconds"`
}

// simulate computes the attempts and outcome for a sequence with a retry-safe request.
func simulate(items []string, maxRetries int) (int, any) {
	for i := 0; ; i++ {
		item := items[minInt(i, len(items)-1)]
		switch item {
		case "conn", "429", "502", "503", "504":
			if i >= maxRetries {
				if item == "conn" {
					return i + 1, "ConnectionError"
				}
				return i + 1, fmt.Sprintf("status=%s", item)
			}
		default:
			if strings.HasPrefix(item, "2") {
				return i + 1, 200
			}
			return i + 1, "status=" + item
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

var outcomeSentinel = map[string]error{
	"UpstreamError": ErrUpstream, "RateLimitError": ErrRateLimit, "NotFoundError": ErrNotFound, "ValidationError": ErrValidation,
	"AuthenticationError": ErrAuthentication, "PermissionError": ErrPermission, "ConflictError": ErrConflict,
	"PaymentRequiredError": ErrPaymentRequired, "TimeoutError": ErrTimeout, "ConnectionError": ErrConnection,
}

func checkOutcome(t *testing.T, err error, outcome any) {
	t.Helper()
	switch o := outcome.(type) {
	case float64, int:
		if err != nil {
			t.Fatalf("want success, got %v", err)
		}
	case string:
		if err == nil {
			t.Fatalf("want %s, got success", o)
		}
		if strings.HasPrefix(o, "status=") {
			var api *APIError
			if !errors.As(err, &api) || fmt.Sprint(api.Status) != strings.TrimPrefix(o, "status=") {
				t.Fatalf("want %s, got %v", o, err)
			}
			return
		}
		name := regexp.MustCompile(`\(.*$`).ReplaceAllString(o, "")
		sentinel, ok := outcomeSentinel[name]
		if !ok || !errors.Is(err, sentinel) {
			t.Fatalf("want %s, got %v", o, err)
		}
		var api *APIError
		if m := regexp.MustCompile(`\((\d{3})\)`).FindStringSubmatch(o); m != nil {
			if !errors.As(err, &api) || fmt.Sprint(api.Status) != m[1] {
				t.Fatalf("want status %s, got %v", m[1], err)
			}
		}
		if m := regexp.MustCompile(`\(retryAfter=(\d+)\)`).FindStringSubmatch(o); m != nil {
			if !errors.As(err, &api) || fmt.Sprint(int(api.RetryAfter/time.Second)) != m[1] {
				t.Fatalf("want retryAfter %s, got %v", m[1], err)
			}
		}
		if m := regexp.MustCompile(`\(([A-Z_]{4,})\)`).FindStringSubmatch(o); m != nil {
			if !errors.As(err, &api) || api.Code != m[1] {
				t.Fatalf("want code %s, got %v", m[1], err)
			}
		}
	default:
		t.Fatalf("unknown outcome %v", outcome)
	}
}

// prune drops nulls, empty strings and empty lists so omitted-vs-null/empty differences do not matter.
func prune(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			if x == nil || x == "" {
				continue
			}
			if a, ok := x.([]any); ok && len(a) == 0 { // an empty list and an absent one are equivalent
				continue
			}
			out[k] = prune(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = prune(x)
		}
		return out
	}
	return v
}

func extraOf(v any) map[string]json.RawMessage {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return nil
		}
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Slice && rv.Len() > 0 {
		rv = rv.Index(0)
	}
	if rv.Kind() != reflect.Struct {
		return nil
	}
	f := rv.FieldByName("Extra")
	if !f.IsValid() {
		return nil
	}
	m, _ := f.Interface().(map[string]json.RawMessage)
	return m
}

// scenarioPolicy returns the retry policy the SDK applies to the operation a scenario exercises.
func scenarioPolicy(s retryScenario) retryPolicy {
	for _, suf := range []string{"/replay", "/cancel", "/replace", "/pan-card", "/topup"} {
		if s.Method == "POST" && strings.HasSuffix(s.Path, suf) {
			return retryNever
		}
	}
	if s.Method == "POST" && s.Path == "/v1/estamp/orders/validate" {
		return retrySafe
	}
	return retryDefault
}

// casesByTemplate maps "METHOD /spec/template" to the SDK call that exercises it.
func casesByTemplate(t *testing.T) map[string]opCase {
	t.Helper()
	ops := loadSpecOps(t, "spec/openapi.json")
	byKey := map[string]opCase{}
	for _, tc := range operationCases() {
		best := ""
		for _, op := range ops {
			if op.Method == tc.Method && templateRegexp(op.Template).MatchString(tc.Path) && (best == "" || strings.Count(op.Template, "{") < strings.Count(best, "{")) {
				best = op.Template
			}
		}
		byKey[tc.Method+" "+best] = tc
	}
	if len(byKey) != 62 {
		t.Fatalf("mapped %d of 62 operations", len(byKey))
	}
	return byKey
}
