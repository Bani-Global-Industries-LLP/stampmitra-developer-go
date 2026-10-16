package stampmitra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestErrorStatusMapping(t *testing.T) {
	tests := []struct {
		status   int
		sentinel error
		is       func(error) bool
	}{
		{400, ErrValidation, IsValidation},
		{401, ErrAuthentication, IsAuthentication},
		{402, ErrPaymentRequired, IsPaymentRequired},
		{403, ErrPermission, IsPermission},
		{404, ErrNotFound, IsNotFound},
		{409, ErrConflict, IsConflict},
		{429, ErrRateLimit, IsRateLimit},
		{500, ErrUpstream, IsUpstream},
		{502, ErrUpstream, IsUpstream},
		{503, ErrUpstream, IsUpstream},
		{504, ErrUpstream, IsUpstream},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
				writeJSON(w, tc.status, `{"error":{"code":"SOME_CODE","message":"server said no","requestId":"req_abc","field":"pan"}}`)
			}, WithMaxRetries(0))
			_, err := env.c.Usage.Summary(context.Background())
			var api *APIError
			if !errors.As(err, &api) {
				t.Fatalf("want *APIError, got %T %v", err, err)
			}
			if api.Status != tc.status || api.Code != "SOME_CODE" || api.Message != "server said no" || api.RequestID != "req_abc" || api.Field != "pan" {
				t.Errorf("fields: %+v", api)
			}
			if !errors.Is(err, tc.sentinel) || !tc.is(err) {
				t.Errorf("sentinel %v not matched", tc.sentinel)
			}
			for _, other := range []error{ErrValidation, ErrAuthentication, ErrPaymentRequired, ErrPermission, ErrNotFound, ErrConflict, ErrRateLimit, ErrUpstream} {
				if other != tc.sentinel && errors.Is(err, other) {
					t.Errorf("also matches %v", other)
				}
			}
			if RequestID(err) != "req_abc" {
				t.Errorf("RequestID() = %q", RequestID(err))
			}
			if strings.Contains(err.Error(), testKey) {
				t.Error("error text leaks the key")
			}
			if !strings.Contains(err.Error(), "req_abc") || !strings.Contains(err.Error(), "SOME_CODE") {
				t.Errorf("Error() = %q", err.Error())
			}
		})
	}
}

func TestErrorWrappedWithFmtErrorf(t *testing.T) {
	err := fmt.Errorf("lookup failed: %w", &APIError{Status: 404, Code: "NOT_FOUND"})
	var api *APIError
	if !errors.As(err, &api) || !IsNotFound(err) {
		t.Fatal("errors.As / IsNotFound must see through wrapping")
	}
}

func TestOtherStatusesHaveNoSentinel(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeErr(w, 418, "TEAPOT", "short and stout") }, WithMaxRetries(0))
	_, err := env.c.Usage.Summary(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.Status != 418 {
		t.Fatalf("got %v", err)
	}
	for _, s := range []error{ErrValidation, ErrNotFound, ErrUpstream, ErrRateLimit} {
		if errors.Is(err, s) {
			t.Errorf("418 must not match %v", s)
		}
	}
}

func TestNonJSONErrorBodiesMapByStatus(t *testing.T) {
	for _, tc := range []struct {
		status int
		is     func(error) bool
	}{{502, IsUpstream}, {429, IsRateLimit}, {404, IsNotFound}, {500, IsUpstream}} {
		env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte("<html>Bad Gateway sk_secret</html>"))
		}, WithMaxRetries(0))
		_, err := env.c.Usage.Summary(context.Background())
		var api *APIError
		if !errors.As(err, &api) || api.Status != tc.status || !tc.is(err) {
			t.Fatalf("status %d: got %v", tc.status, err)
		}
		if api.Code != "" || api.Message != fmt.Sprintf("HTTP %d", tc.status) {
			t.Errorf("fallback message: %+v", api)
		}
		if strings.Contains(api.Error(), "<html>") {
			t.Errorf("raw body must not be in Error(): %s", api.Error())
		}
	}
	// Empty body and JSON that is not the envelope.
	for _, body := range []string{"", "{}", "[]", `{"error":"flat string"}`, `{"error":null}`} {
		env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 503, body) }, WithMaxRetries(0))
		_, err := env.c.Usage.Summary(context.Background())
		if !IsUpstream(err) {
			t.Errorf("body %q: %v", body, err)
		}
	}
}

func TestPermissionMessageVerbatim(t *testing.T) {
	const msg = "Your IP 203.0.113.9 is not allowlisted for sensitive verifications"
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeErr(w, 403, "FORBIDDEN", msg) })
	_, err := env.c.Verify.Passport(context.Background(), &PassportVerifyParams{FileNumber: "PA1079341954215", DOB: "1990-01-31"})
	var api *APIError
	if !errors.As(err, &api) || api.Message != msg || !IsPermission(err) {
		t.Fatalf("got %v", err)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"", 0}, {"0", 0}, {"-5", 0}, {"abc", 0}, {"2", 2 * time.Second}, {"1.5", 1500 * time.Millisecond},
		{"3600", 60 * time.Second}, {"NaN", 0}, {"Inf", 0},
		{now.Add(10 * time.Second).Format(http.TimeFormat), 10 * time.Second},
		{now.Add(-10 * time.Second).Format(http.TimeFormat), 0},
		{now.Add(time.Hour).Format(http.TimeFormat), 60 * time.Second},
	}
	for _, tc := range tests {
		if got := parseRetryAfter(tc.in, now); got != tc.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestRateLimitExposesRetryAfter(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		writeErr(w, 429, "TOO_MANY_REQUESTS", "slow down")
	}, WithMaxRetries(0))
	_, err := env.c.Usage.Summary(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.RetryAfter != 7*time.Second || !IsRateLimit(err) {
		t.Fatalf("got %+v", api)
	}
}

func TestDecodeErrorOnBadSuccessBody(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, "not json") })
	_, err := env.c.Usage.Summary(context.Background())
	if !errors.Is(err, ErrDecode) {
		t.Fatalf("got %v", err)
	}
	var de *DecodeError
	if !errors.As(err, &de) || de.Status != 200 {
		t.Fatalf("got %v", err)
	}
	if len(env.rec.all()) != 1 {
		t.Fatal("a decode failure must not be retried")
	}
}

func TestRawResponse(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "hdr-id")
		writeJSON(w, 200, `{"externalOrderId":"o1","status":"CREATED"}`)
	})
	var raw Response
	st, err := env.c.Orders.GetStatus(context.Background(), "o1", WithRawResponse(&raw))
	if err != nil || st.ExternalOrderID != "o1" {
		t.Fatal(err)
	}
	if raw.StatusCode != 200 || raw.RequestID != "hdr-id" || raw.Attempts != 1 || !strings.Contains(string(raw.Body), "CREATED") || raw.Header.Get("Content-Type") == "" {
		t.Errorf("raw = %+v", raw)
	}

	env2 := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeErr(w, 404, "NOT_FOUND", "nope") })
	var raw2 Response
	_, err = env2.c.Orders.GetStatus(context.Background(), "zzz", WithRawResponse(&raw2))
	if !IsNotFound(err) || raw2.StatusCode != 404 || raw2.RequestID != "req_123" {
		t.Errorf("raw on error = %+v, err %v", raw2, err)
	}
}
