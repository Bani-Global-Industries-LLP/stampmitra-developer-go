//go:build conformance

package stampmitra

// Black-box conformance run against the spec repo's mock server (scripts/mock-server.mjs in
// stampmitra-developer-openapi). TestMain starts it with os/exec on a free port.
//
//	STAMPMITRA_SPEC_DIR=/path/to/stampmitra-developer-openapi go test -tags conformance -run Conformance ./...
//
// STAMPMITRA_SPEC_DIR defaults to /home/user/stampmitra-developer-openapi. Node must be on PATH
// (or at /opt/node22/bin/node). Set STAMPMITRA_MOCK_URL to use an already running mock instead.
// Without a spec checkout the tests are skipped, unless CONFORMANCE_REQUIRED=1 (CI), where they fail.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

var (
	mockURL  string
	mockSkip string
)

func TestMain(m *testing.M) {
	code := func() int {
		stop, err := startMock()
		if err != nil {
			if os.Getenv("CONFORMANCE_REQUIRED") == "1" {
				fmt.Fprintln(os.Stderr, "conformance mock server unavailable:", err)
				return 1
			}
			mockSkip = err.Error()
		}
		if stop != nil {
			defer stop()
		}
		return m.Run()
	}()
	os.Exit(code)
}

func startMock() (func(), error) {
	if u := os.Getenv("STAMPMITRA_MOCK_URL"); u != "" {
		mockURL = strings.TrimRight(u, "/")
		return nil, waitHealthy(mockURL, 10*time.Second)
	}
	dir := specDir()
	if dir == "" {
		return nil, errors.New("no stampmitra-developer-openapi checkout (set STAMPMITRA_SPEC_DIR)")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		if _, serr := os.Stat("/opt/node22/bin/node"); serr != nil {
			return nil, errors.New("node not found on PATH")
		}
		node = "/opt/node22/bin/node"
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	cmd := exec.Command(node, filepath.Join(dir, "scripts", "mock-server.mjs"), "--port", fmt.Sprint(port), "--host", "127.0.0.1")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	stop := func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }
	mockURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitHealthy(mockURL, 15*time.Second); err != nil {
		stop()
		return nil, fmt.Errorf("%w; output: %s", err, out.String())
	}
	return stop, nil
}

func waitHealthy(base string, d time.Duration) error {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/__mock/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == 200 {
				return nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("mock server did not become healthy")
}

type mockRequest struct {
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
	Body    json.RawMessage   `json:"body"`
}

type confEnv struct {
	t      *testing.T
	c      *Client
	sleeps []time.Duration
}

func conf(t *testing.T, opts ...Option) *confEnv {
	t.Helper()
	if mockURL == "" {
		if os.Getenv("CONFORMANCE_REQUIRED") == "1" {
			t.Fatal("mock server required")
		}
		t.Skip("conformance skipped: " + mockSkip)
	}
	resp, err := http.Post(mockURL+"/__mock/reset", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	// The SDK client gets its own transport, with keep-alives off. Sharing http.DefaultTransport with the
	// helper calls above lets a pooled idle connection be reused; when the mock then destroys that
	// connection, net/http transparently replays the idempotent GET on a fresh one, silently consuming a
	// scripted "conn" step and making retry counts depend on timing (observed on GitHub runners).
	tr := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(tr.CloseIdleConnections)
	base := []Option{WithBaseURL(mockURL), WithHTTPClient(&http.Client{Transport: tr})}
	c, err := NewClient(testKey, append(base, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	e := &confEnv{t: t, c: c}
	c.sleep = func(ctx context.Context, d time.Duration) error { e.sleeps = append(e.sleeps, d); return ctx.Err() }
	return e
}

func (e *confEnv) recorded() []mockRequest {
	e.t.Helper()
	resp, err := http.Get(mockURL + "/__mock/requests")
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out []mockRequest
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		e.t.Fatal(err)
	}
	return out
}

func scenario(s string) RequestOption { return WithRequestHeader("x-mock-scenario", s) }

// ---- all 31 operations ------------------------------------------------------------------------

func TestConformanceEveryOperation(t *testing.T) {
	cases := operationCases()
	if len(cases) != 31 {
		t.Fatalf("%d operations", len(cases))
	}
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			e := conf(t)
			res, err := tc.Call(context.Background(), e.c)
			if err != nil {
				t.Fatal(err)
			}
			if res == nil {
				t.Fatal("nil result")
			}
			reqs := e.recorded()
			if len(reqs) != 1 || reqs[0].Method != tc.Method || reqs[0].Path != tc.Path {
				t.Fatalf("recorded %+v", reqs)
			}
			h := reqs[0].Headers
			if h["authorization"] != "Bearer "+testKey || !strings.HasPrefix(h["user-agent"], "stampmitra-go/"+Version) || h["accept"] != "application/json" {
				t.Errorf("headers: %v", h)
			}
			if tc.Method == "POST" && !tc.NoKey && !uuid.MatchString(h["idempotency-key"]) {
				t.Errorf("idempotency-key = %q", h["idempotency-key"])
			}
			if tc.Method == "GET" && h["idempotency-key"] != "" {
				t.Errorf("GET must not carry an idempotency key")
			}
		})
	}
}

// TestConformanceFixtureBodiesRoundTrip decodes each typed success fixture and re-encodes it: any
// field the models do not declare would still be there (Extra), and none may be altered or lost.
func TestConformanceFixtureBodiesRoundTrip(t *testing.T) {
	dir := specDir()
	if dir == "" {
		t.Skip("no spec checkout")
	}
	for _, tc := range operationCases() {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			e := conf(t)
			res, err := tc.Call(context.Background(), e.c)
			if err != nil {
				t.Fatal(err)
			}
			name := fixtureName(tc.Method, tc.Path)
			raw, err := os.ReadFile(filepath.Join(dir, "fixtures", "operations", name+".success.json"))
			if err != nil {
				t.Skip("no fixture " + name)
			}
			var fx struct {
				Response struct {
					Body json.RawMessage `json:"body"`
				} `json:"response"`
			}
			_ = json.Unmarshal(raw, &fx)
			out, err := json.Marshal(res)
			if err != nil {
				t.Fatal(err)
			}
			var want, got any
			_ = json.Unmarshal(fx.Response.Body, &want)
			_ = json.Unmarshal(out, &got)
			if !reflect.DeepEqual(prune(want), prune(got)) {
				t.Errorf("round trip changed the body:\n want %s\n  got %s", fx.Response.Body, out)
			}
			// typed models should cover every fixture field: nothing should land in Extra
			if ex := extraOf(res); len(ex) > 0 {
				t.Errorf("fixture fields not modelled: %v", ex)
			}
		})
	}
}

func fixtureName(method, path string) string {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	// path params in the table are concrete; fixtures use template names, so map the known ones
	repl := map[string]string{"ord_123": "orderid", "o1": "id", "doc_1": "documentid", "pi_1": "id", "evt_1": "id", "MH": "statecode", "rent_agreement": "instrumentcode"}
	for i, s := range segs {
		if r, ok := repl[s]; ok {
			// o1 is documentid? no: first occurrence under /orders is id
			segs[i] = r
		} else if strings.HasPrefix(s, "estamp.") {
			segs[i] = "servicecode"
		}
	}
	return strings.ToLower(method) + "-" + strings.Join(segs, "-")
}

// prune drops nulls and empty strings so omitted-vs-null/empty differences do not matter.
func prune(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			if x == nil || x == "" {
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

// ---- errors -----------------------------------------------------------------------------------

func TestConformanceErrorStatuses(t *testing.T) {
	tests := []struct {
		status   int
		sentinel error
	}{
		{400, ErrValidation}, {401, ErrAuthentication}, {402, ErrPaymentRequired}, {403, ErrPermission}, {404, ErrNotFound},
		{409, ErrConflict}, {429, ErrRateLimit}, {500, ErrUpstream}, {502, ErrUpstream}, {503, ErrUpstream}, {504, ErrUpstream},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			e := conf(t, WithMaxRetries(0))
			_, err := e.c.Usage.Summary(context.Background(), scenario(fmt.Sprintf("error-%d", tc.status)))
			var api *APIError
			if !errors.As(err, &api) || api.Status != tc.status || !errors.Is(err, tc.sentinel) {
				t.Fatalf("got %v", err)
			}
			if api.Code == "" || api.RequestID == "" || api.Message == "" {
				t.Errorf("incomplete error: %+v", api)
			}
			if strings.Contains(err.Error(), testKey) {
				t.Error("key leaked")
			}
		})
	}
}

func TestConformanceRateLimitHeaders(t *testing.T) {
	e := conf(t, WithMaxRetries(0))
	_, err := e.c.Usage.Summary(context.Background(), scenario("retry-after:7"))
	var api *APIError
	if !errors.As(err, &api) || api.RetryAfter != 7*time.Second || !IsRateLimit(err) {
		t.Fatalf("with Retry-After: %v", err)
	}

	// documented 429 fixture, served without Retry-After: computed backoff, RetryAfter is zero
	e = conf(t, WithMaxRetries(1))
	_, err = e.c.Estamp.ListStates(context.Background(), scenario("error-429"))
	if !errors.As(err, &api) || api.RetryAfter != 0 || !IsRateLimit(err) {
		t.Fatalf("without Retry-After: %v (%+v)", err, api)
	}
	if len(e.sleeps) != 1 || e.sleeps[0] > 500*time.Millisecond {
		t.Fatalf("backoff = %v", e.sleeps)
	}

	// Retry-After beats the computed backoff
	e = conf(t, WithMaxRetries(1))
	_, err = e.c.Usage.Summary(context.Background(), scenario("retry-after:3"))
	if !IsRateLimit(err) || len(e.sleeps) != 1 || e.sleeps[0] != 3*time.Second {
		t.Fatalf("sleeps %v err %v", e.sleeps, err)
	}
	if n := len(e.recorded()); n != 2 {
		t.Fatalf("attempts = %d", n)
	}
}

// ---- shared retry scenarios ---------------------------------------------------------------------

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
}

func TestConformanceSharedRetryScenarios(t *testing.T) {
	raw, err := os.ReadFile("testdata/retry-scenarios.json")
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Scenarios []retryScenario `json:"scenarios"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Scenarios) < 25 {
		t.Fatalf("only %d scenarios", len(f.Scenarios))
	}
	for _, s := range f.Scenarios {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			var items []string
			for _, it := range s.Sequence {
				switch v := it.(type) {
				case string:
					if v == "timeout" {
						t.Skip("client-side timeout step: covered by TestConformanceTimeout*")
					}
					items = append(items, "conn")
				case float64:
					items = append(items, fmt.Sprint(int(v)))
				case map[string]any:
					items = append(items, fmt.Sprint(int(v["status"].(float64))))
				}
			}
			maxRetries := 2
			if s.MaxRetries != nil {
				maxRetries = *s.MaxRetries
			}
			e := conf(t, WithMaxRetries(maxRetries))
			isReplay := strings.HasSuffix(s.Path, "/replay")
			opts := []RequestOption{scenario("seq:" + strings.ReplaceAll(s.Name, ":", "_") + ":" + strings.Join(items, ","))}
			if string(s.IdempotencyKey) == "true" {
				opts = append(opts, WithIdempotencyKey("caller-key-1"))
			}
			autoKey := s.Method == "POST" && !isReplay
			var body any
			if s.Method == "POST" {
				body = map[string]any{}
			}

			// Wave-2 behaviour: every POST except webhook replay carries an auto idempotency key, so
			// scenarios that expect a keyless POST to stay single-attempt are evaluated as "keyed".
			wantAttempts := s.ExpectedAttempts
			wantOutcome := s.Outcome
			keyed := string(s.IdempotencyKey) != "false" || autoKey
			if s.Method == "POST" && !isReplay && string(s.IdempotencyKey) == "false" {
				wantAttempts, wantOutcome = simulate(items, maxRetries)
			}
			_ = keyed

			var out GenericResponse
			err := e.c.do(context.Background(), s.Method, s.Path, nil, body, &out, autoKey, opts)
			checkOutcome(t, err, wantOutcome)

			var mine []mockRequest
			for _, r := range e.recorded() {
				if r.Path == s.Path {
					mine = append(mine, r)
				}
			}
			if len(mine) != wantAttempts {
				t.Fatalf("attempts = %d, want %d", len(mine), wantAttempts)
			}
			if s.SameIdempotencyKeyAcrossAlways || (autoKey && len(mine) > 1) {
				k := mine[0].Headers["idempotency-key"]
				if k == "" {
					t.Fatal("missing idempotency key")
				}
				for _, r := range mine {
					if r.Headers["idempotency-key"] != k {
						t.Fatal("idempotency key changed across retries")
					}
				}
			}
			if s.Method == "GET" {
				for _, r := range mine {
					if r.Headers["idempotency-key"] != "" {
						t.Fatal("GET must not carry a key")
					}
				}
			}
			if isReplay && string(s.IdempotencyKey) == "false" && mine[0].Headers["idempotency-key"] != "" {
				t.Fatal("replay must not carry an automatic key")
			}
		})
	}
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
	"TimeoutError": ErrTimeout, "ConnectionError": ErrConnection,
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
		if m := regexp.MustCompile(`\((\d{3})\)`).FindStringSubmatch(o); m != nil {
			var api *APIError
			if !errors.As(err, &api) || fmt.Sprint(api.Status) != m[1] {
				t.Fatalf("want status %s, got %v", m[1], err)
			}
		}
	default:
		t.Fatalf("unknown outcome %v", outcome)
	}
}

// ---- idempotency through the real resource methods -----------------------------------------------

func TestConformanceIdempotencyKeyReuse(t *testing.T) {
	e := conf(t)
	if _, err := e.c.Verify.PAN(context.Background(), &PANVerifyParams{PAN: "ABCDE1234F"}, scenario("seq:idem:503,502,200")); err != nil {
		t.Fatal(err)
	}
	reqs := e.recorded()
	if len(reqs) != 3 {
		t.Fatalf("attempts = %d", len(reqs))
	}
	k := reqs[0].Headers["idempotency-key"]
	for _, r := range reqs {
		if k == "" || r.Headers["idempotency-key"] != k {
			t.Fatalf("keys differ: %v", reqs)
		}
	}

	e = conf(t)
	_, err := e.c.Orders.Create(context.Background(), &CreateOrderParams{ExternalOrderID: "o", ExternalCustomerID: "c", ServiceCode: "s"},
		WithIdempotencyKey("mine-1"), scenario("seq:idem2:503,201"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range e.recorded() {
		if r.Headers["idempotency-key"] != "mine-1" {
			t.Fatalf("key = %q", r.Headers["idempotency-key"])
		}
	}
}

func TestConformanceNoRetryOn4xxPost(t *testing.T) {
	for _, st := range []int{400, 401, 402, 403, 404, 409} {
		e := conf(t)
		_, err := e.c.Verify.PAN(context.Background(), &PANVerifyParams{PAN: "ABCDE1234F"}, scenario(fmt.Sprintf("seq:x%d:%d,200", st, st)))
		if err == nil {
			t.Fatal("want error")
		}
		if n := len(e.recorded()); n != 1 {
			t.Errorf("%d: attempts = %d", st, n)
		}
	}
}

// ---- transport failures ---------------------------------------------------------------------------

func TestConformanceTimeout(t *testing.T) {
	e := conf(t, WithMaxRetries(0), WithTimeout(30*time.Second))
	start := time.Now()
	_, err := e.c.Usage.Summary(context.Background(), WithRequestTimeout(300*time.Millisecond), scenario("delay:1500"))
	if !IsTimeout(err) || !IsConnection(err) {
		t.Fatalf("got %v", err)
	}
	if time.Since(start) > 1200*time.Millisecond {
		t.Fatal("per-call timeout did not override the 30s default")
	}
}

func TestConformanceTimeoutRetriedForGetThenReported(t *testing.T) {
	e := conf(t, WithMaxRetries(1))
	_, err := e.c.Usage.Summary(context.Background(), WithRequestTimeout(200*time.Millisecond), scenario("delay:1000"))
	if !IsTimeout(err) {
		t.Fatalf("got %v", err)
	}
	if n := len(e.recorded()); n != 2 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestConformancePostTimeoutWithKeyIsRetriedWithSameKey(t *testing.T) {
	e := conf(t, WithMaxRetries(1))
	_, err := e.c.Orders.Create(context.Background(), &CreateOrderParams{ExternalOrderID: "o", ExternalCustomerID: "c", ServiceCode: "s"},
		WithRequestTimeout(200*time.Millisecond), scenario("delay:1000"))
	if !IsTimeout(err) {
		t.Fatalf("got %v", err)
	}
	reqs := e.recorded()
	if len(reqs) != 2 || reqs[0].Headers["idempotency-key"] == "" || reqs[0].Headers["idempotency-key"] != reqs[1].Headers["idempotency-key"] {
		t.Fatalf("reqs: %+v", reqs)
	}
}

func TestConformanceConnectionRefused(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := l.Addr().String()
	_ = l.Close()
	c, _ := NewClient(testKey, WithBaseURL("http://"+addr), WithMaxRetries(1))
	var sleeps []time.Duration
	c.sleep = func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	_, err := c.Usage.Summary(context.Background())
	if !IsConnection(err) || IsTimeout(err) || len(sleeps) != 1 || strings.Contains(err.Error(), testKey) {
		t.Fatalf("got %v sleeps %v", err, sleeps)
	}
}

func TestConformanceConnectionDroppedMidRequest(t *testing.T) {
	e := conf(t)
	if _, err := e.c.Usage.Summary(context.Background(), scenario("seq:drop:conn,200")); err != nil {
		t.Fatal(err)
	}
	if n := len(e.recorded()); n != 2 {
		t.Fatalf("attempts = %d", n)
	}
	e = conf(t, WithMaxRetries(1))
	_, err := e.c.Usage.Summary(context.Background(), scenario("seq:drop2:conn,conn,200"))
	if !IsConnection(err) || IsTimeout(err) {
		t.Fatalf("got %v", err)
	}
}

// ---- bodies the mock cannot produce ---------------------------------------------------------------

func TestConformanceNonJSONErrorBodies(t *testing.T) {
	for _, st := range []int{502, 429, 404, 500, 403} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(st)
			_, _ = io.WriteString(w, "<html>Bad Gateway</html>")
		}))
		c, _ := NewClient(testKey, WithBaseURL(srv.URL), WithMaxRetries(0))
		_, err := c.Usage.Summary(context.Background())
		srv.Close()
		var api *APIError
		if !errors.As(err, &api) || api.Status != st || sentinelForStatus(st) == nil || !errors.Is(err, sentinelForStatus(st)) {
			t.Fatalf("%d: %v", st, err)
		}
	}
}

func TestConformanceUnknownResponseFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"referenceId":"SMV-1","valid":true,"registeredName":"ACME","brandNewField":{"x":1},"anotherNew":[1]}`)
	}))
	defer srv.Close()
	c, _ := NewClient(testKey, WithBaseURL(srv.URL))
	res, err := c.Verify.PAN(context.Background(), &PANVerifyParams{PAN: "ABCDE1234F"})
	if err != nil || !res.Valid || string(res.Extra["brandNewField"]) != `{"x":1}` || len(res.Extra) != 2 {
		t.Fatalf("%v %+v", err, res)
	}
}

func TestConformanceThreePagePagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("cursor") {
		case "":
			writeJSON(w, 200, page([]string{"a"}, "c1"))
		case "c1":
			writeJSON(w, 200, page([]string{"b"}, "c2"))
		case "c2":
			writeJSON(w, 200, page([]string{"c"}, ""))
		}
	}))
	defer srv.Close()
	c, _ := NewClient(testKey, WithBaseURL(srv.URL))
	it := c.Logs.ListAll(&LogsListParams{Limit: 1})
	var ids []string
	for it.Next(context.Background()) {
		ids = append(ids, it.Log().ID)
	}
	if it.Err() != nil || strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("ids %v err %v", ids, it.Err())
	}
}

func TestConformanceLogsPaginationAgainstMock(t *testing.T) {
	e := conf(t)
	it := e.c.Logs.ListAll(&LogsListParams{Limit: 50})
	n := 0
	for it.Next(context.Background()) {
		n++
	}
	if it.Err() != nil || n == 0 || len(e.recorded()) != 1 {
		t.Fatalf("n=%d err=%v requests=%d", n, it.Err(), len(e.recorded()))
	}
}

// ---- shared vectors stay in sync with the spec repo -----------------------------------------------

func TestConformanceVendoredVectorsMatchUpstream(t *testing.T) {
	dir := specDir()
	if dir == "" {
		t.Skip("no spec checkout")
	}
	for _, name := range []string{"validators.json", "webhook-signatures.json", "retry-scenarios.json"} {
		up, err := os.ReadFile(filepath.Join(dir, "fixtures", name))
		if err != nil {
			t.Fatal(err)
		}
		local, _ := os.ReadFile(filepath.Join("testdata", name))
		if !bytes.Equal(up, local) {
			t.Errorf("testdata/%s differs from the spec repo's fixtures/%s", name, name)
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "fixtures", "webhooks", "*.json"))
	if len(files) != 8 {
		t.Fatalf("%d upstream webhook fixtures", len(files))
	}
	for _, f := range files {
		up, _ := os.ReadFile(f)
		local, err := os.ReadFile(filepath.Join("testdata", "webhooks", filepath.Base(f)))
		if err != nil || !bytes.Equal(up, local) {
			t.Errorf("webhook fixture %s out of sync", filepath.Base(f))
		}
	}
}
