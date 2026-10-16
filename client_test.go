package stampmitra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientKeyValidation(t *testing.T) {
	t.Setenv(EnvAPIKey, "")
	tests := []struct {
		name    string
		key     string
		opts    []Option
		wantErr bool
		env     string
	}{
		{"test key", "sk_test_abcdefgh", nil, false, "test"},
		{"live key", "sk_live_abcdefgh_-12", nil, false, "live"},
		{"too short", "sk_test_abc", nil, true, ""},
		{"wrong prefix", "pk_test_abcdefgh", nil, true, ""},
		{"empty", "", nil, true, ""},
		{"custom allowed", "custom-key-1", []Option{WithAllowCustomKey()}, false, "custom"},
		{"whitespace", "sk_test_abcdefgh ", nil, true, ""},
		{"newline injection", "sk_test_abcdefgh\nX-Evil: 1", []Option{WithAllowCustomKey()}, true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewClient(tc.key, tc.opts...)
			if tc.wantErr {
				if err == nil || !errors.Is(err, ErrConfig) {
					t.Fatalf("want ErrConfig, got %v", err)
				}
				if tc.key != "" && strings.Contains(err.Error(), tc.key) {
					t.Fatalf("error message leaks the key: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if c.Environment != tc.env {
				t.Fatalf("environment = %q, want %q", c.Environment, tc.env)
			}
		})
	}
}

func TestNewClientReadsEnv(t *testing.T) {
	t.Setenv(EnvAPIKey, "sk_live_envkey12345")
	c, err := NewClient("")
	if err != nil {
		t.Fatal(err)
	}
	if c.Environment != "live" {
		t.Fatalf("got %q", c.Environment)
	}
}

func TestBaseURLValidation(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://api-b2b.stampmitra.in", true},
		{"https://api-b2b.stampmitra.in/", true},
		{"https://example.com/prefix", true},
		{"http://localhost:4010", true},
		{"http://127.0.0.1:4010", true},
		{"http://[::1]:4010", true},
		{"http://api-b2b.stampmitra.in", false},
		{"http://localhost.evil.com", false},
		{"ftp://example.com", false},
		{"https://user:pass@example.com", false},
		{"https://user@example.com", false},
		{"https://example.com?x=1", false},
		{"https://example.com#frag", false},
		{"https://", false},
		{"example.com", false},
		{"", false},
	}
	for _, tc := range tests {
		_, err := NewClient(testKey, WithBaseURL(tc.url))
		if tc.ok && err != nil {
			t.Errorf("%q: unexpected error %v", tc.url, err)
		}
		if !tc.ok {
			if err == nil {
				t.Errorf("%q: expected error", tc.url)
			} else if strings.Contains(err.Error(), "pass") {
				t.Errorf("%q: error leaks credentials: %v", tc.url, err)
			}
		}
	}
}

func TestOptionValidation(t *testing.T) {
	for name, o := range map[string]Option{
		"negative retries": WithMaxRetries(-1),
		"zero timeout":     WithTimeout(0),
		"bad header":       WithDefaultHeader("X-A", "b\r\nInjected: 1"),
		"auth header":      WithDefaultHeader("Authorization", "Bearer x"),
	} {
		if _, err := NewClient(testKey, o); !errors.Is(err, ErrConfig) {
			t.Errorf("%s: want ErrConfig, got %v", name, err)
		}
	}
}

func TestClientNeverPrintsKey(t *testing.T) {
	c, err := NewClient(testKey, WithAppInfo(AppInfo{Name: "x"}))
	if err != nil {
		t.Fatal(err)
	}
	outs := []string{
		fmt.Sprint(c), fmt.Sprint(*c), fmt.Sprintf("%v %+v %#v %s %q %x", c, c, c, c, c, c),
		fmt.Sprintf("%v %+v %#v", *c, *c, *c),
		fmt.Sprintf("%+v %#v", struct{ C Client }{*c}, struct{ C *Client }{c}),
		fmt.Sprintf("%v %+v", c.apiKey, c.apiKey), fmt.Sprintf("%#v", c.apiKey),
		fmt.Sprintf("%+v", c.Verify), fmt.Sprintf("%+v", c.Orders),
	}
	for i, o := range outs {
		if strings.Contains(o, testKey) || strings.Contains(o, "abcdefgh12345678") {
			t.Errorf("output %d leaks the API key: %s", i, o)
		}
	}
	if !strings.Contains(c.String(), "[REDACTED]") {
		t.Errorf("String() = %q", c.String())
	}
}

func TestRequestHeaders(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"currentPeriod":{"start":"a","end":"b"},"totalApiRequests":1,"callsByService":{},"callsByPeriod":[],"callsByDay":[]}`)
	}, WithAppInfo(AppInfo{Name: "myapp", Version: "1.2.3", URL: "https://example.com"}), WithDefaultHeader("X-Trace", "t1"))
	if _, err := env.c.Usage.Summary(context.Background(), WithRequestHeader("X-Call", "c1")); err != nil {
		t.Fatal(err)
	}
	r := env.rec.all()[0]
	if got := r.Header.Get("Authorization"); got != "Bearer "+testKey {
		t.Errorf("Authorization = %q", got)
	}
	if r.Header.Get("Accept") != "application/json" || r.Header.Get("X-Trace") != "t1" || r.Header.Get("X-Call") != "c1" {
		t.Errorf("headers: %v", r.Header)
	}
	ua := r.Header.Get("User-Agent")
	if !strings.HasPrefix(ua, "myapp/1.2.3 (https://example.com) stampmitra-go/"+Version+" (go") {
		t.Errorf("User-Agent = %q", ua)
	}
	if r.Header.Get("Idempotency-Key") != "" || r.Header.Get("Content-Type") != "" {
		t.Errorf("GET must not carry Idempotency-Key/Content-Type: %v", r.Header)
	}
}

func TestPerCallHeadersCannotOverrideAuth(t *testing.T) {
	env := newEnv(t, func(int, http.ResponseWriter, *http.Request) { t.Error("must not reach the network") })
	for _, h := range []string{"Authorization", "x-api-key", "Host", "Content-Length"} {
		_, err := env.c.Usage.Summary(context.Background(), WithRequestHeader(h, "evil"))
		if !errors.Is(err, ErrValidation) {
			t.Errorf("%s: want validation error, got %v", h, err)
		}
	}
	_, err := env.c.Usage.Summary(context.Background(), WithRequestHeader("X-A", "b\r\nX-B: c"))
	if !errors.Is(err, ErrValidation) {
		t.Errorf("CRLF header value: got %v", err)
	}
	_, err = env.c.Orders.Create(context.Background(), &CreateOrderParams{ExternalOrderID: "o", ExternalCustomerID: "c", ServiceCode: "s"}, WithIdempotencyKey("a\nb"))
	if !errors.Is(err, ErrValidation) {
		t.Errorf("CRLF idempotency key: got %v", err)
	}
	if n := len(env.rec.all()); n != 0 {
		t.Errorf("%d requests reached the server", n)
	}
}

func TestRedirectsAreNotFollowed(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("redirect target was contacted with headers %v", r.Header)
	}))
	defer other.Close()
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/steal", http.StatusFound)
	})
	_, err := env.c.Usage.Summary(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.Status != http.StatusFound {
		t.Fatalf("want *APIError with status 302, got %v", err)
	}
	if len(env.rec.all()) != 1 {
		t.Fatal("redirect must not be retried or followed")
	}
}

func TestCustomHTTPClientKeepsTransportButNotRedirects(t *testing.T) {
	called := false
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return http.DefaultTransport.RoundTrip(r)
	})
	orig := &http.Client{Transport: rt, CheckRedirect: func(*http.Request, []*http.Request) error { return nil }}
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"currentPeriod":{"start":"a","end":"b"},"totalApiRequests":1,"callsByService":{},"callsByPeriod":[],"callsByDay":[]}`)
	}, WithHTTPClient(orig))
	if _, err := env.c.Usage.Summary(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("custom transport not used")
	}
	if orig.CheckRedirect == nil || env.c.http.CheckRedirect == nil {
		t.Fatal("unexpected nil CheckRedirect")
	}
	if err := env.c.http.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("SDK client must not follow redirects, got %v", err)
	}
	if err := orig.CheckRedirect(nil, nil); err != nil {
		t.Fatal("the caller's client must not be modified")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestNilContextAndNilParams(t *testing.T) {
	env := newEnv(t, func(int, http.ResponseWriter, *http.Request) {})
	var nilCtx context.Context // deliberately nil
	if _, err := env.c.Usage.Summary(nilCtx); !errors.Is(err, ErrValidation) {
		t.Errorf("nil ctx: %v", err)
	}
	if _, err := env.c.Verify.PAN(context.Background(), nil); !errors.Is(err, ErrValidation) {
		t.Errorf("nil params: %v", err)
	}
}

func TestUUIDv4(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		u, err := newUUIDv4()
		if err != nil {
			t.Fatal(err)
		}
		if len(u) != 36 || u[14] != '4' || !strings.ContainsRune("89ab", rune(u[19])) {
			t.Fatalf("not a v4 UUID: %s", u)
		}
		if seen[u] {
			t.Fatal("duplicate UUID")
		}
		seen[u] = true
	}
}

func TestPathIDValidation(t *testing.T) {
	env := newEnv(t, func(int, http.ResponseWriter, *http.Request) { t.Error("must not reach the network") })
	bad := []string{"", " ", ".", "..", "a/b", "../x", "a\\b", "a?b", "a#b", "a%2fb", "a\nb", "a\x00b"}
	for _, id := range bad {
		if _, err := env.c.Orders.Get(context.Background(), id); !errors.Is(err, ErrValidation) {
			t.Errorf("Orders.Get(%q): want validation error, got %v", id, err)
		}
		if _, err := env.c.Estamp.ListInstruments(context.Background(), id); !errors.Is(err, ErrValidation) {
			t.Errorf("ListInstruments(%q): %v", id, err)
		}
	}
}

func TestPathIDEncoding(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"externalOrderId":"x","status":"CREATED"}`)
	})
	if _, err := env.c.Orders.GetStatus(context.Background(), "ord 1&x=y"); err != nil {
		t.Fatal(err)
	}
	if got := env.rec.all()[0].Path; got != "/v1/orders/ord 1&x=y/status" {
		t.Errorf("decoded path = %q", got)
	}
}

func TestSpecialTimeoutOption(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}, WithMaxRetries(0))
	start := time.Now()
	_, err := env.c.Usage.Summary(context.Background(), WithRequestTimeout(100*time.Millisecond))
	if !errors.Is(err, ErrTimeout) || !errors.Is(err, ErrConnection) {
		t.Fatalf("want timeout, got %v", err)
	}
	var ce *ConnectionError
	if !errors.As(err, &ce) || !ce.Timeout {
		t.Fatalf("want *ConnectionError{Timeout}, got %T", err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("per-call timeout not honoured")
	}
}

func TestContextCancellationIsNotWrappedOrRetried(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		cancel()
		<-r.Context().Done()
	})
	_, err := env.c.Usage.Summary(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if IsConnection(err) {
		t.Fatal("cancellation must not look like a connection error")
	}
	if len(env.rec.all()) != 1 {
		t.Fatal("cancelled call must not be retried")
	}
}

func TestConcurrentUse(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			writeJSON(w, 201, orderJSON)
			return
		}
		writeJSON(w, 200, `{"externalOrderId":"o1","status":"CREATED"}`)
	})
	done := make(chan error, 40)
	for i := 0; i < 40; i++ {
		go func(i int) {
			var err error
			if i%2 == 0 {
				_, err = env.c.Orders.Create(context.Background(), &CreateOrderParams{ExternalOrderID: "o", ExternalCustomerID: "c", ServiceCode: "s"})
			} else {
				_, err = env.c.Orders.GetStatus(context.Background(), "o1")
			}
			done <- err
		}(i)
	}
	for i := 0; i < 40; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	keys := map[string]bool{}
	for _, r := range env.rec.all() {
		if k := r.Header.Get("Idempotency-Key"); k != "" {
			if keys[k] {
				t.Fatal("concurrent calls shared an idempotency key")
			}
			keys[k] = true
		}
	}
}
