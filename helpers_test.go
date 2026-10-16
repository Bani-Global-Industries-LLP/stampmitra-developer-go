package stampmitra

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

const testKey = "sk_test_abcdefgh12345678"

type recordedReq struct {
	Method string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

type recorder struct {
	mu   sync.Mutex
	reqs []recordedReq
}

func (r *recorder) all() []recordedReq {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedReq(nil), r.reqs...)
}

// testEnv is a client wired to an httptest server, with sleeping stubbed out.
type testEnv struct {
	c      *Client
	srv    *httptest.Server
	rec    *recorder
	sleeps []time.Duration
}

// newEnv starts a server whose responses are produced by respond(n, r) where n is the 0-based request index.
func newEnv(t *testing.T, respond func(n int, w http.ResponseWriter, r *http.Request), opts ...Option) *testEnv {
	t.Helper()
	rec := &recorder{}
	var n int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		idx := n
		n++
		mu.Unlock()
		rec.mu.Lock()
		rec.reqs = append(rec.reqs, recordedReq{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Clone(), string(b)})
		rec.mu.Unlock()
		respond(idx, w, r)
	}))
	t.Cleanup(srv.Close)
	all := append([]Option{WithBaseURL(srv.URL)}, opts...)
	c, err := NewClient(testKey, all...)
	if err != nil {
		t.Fatal(err)
	}
	env := &testEnv{c: c, srv: srv, rec: rec}
	c.sleep = func(ctx context.Context, d time.Duration) error {
		env.sleeps = append(env.sleeps, d)
		return ctx.Err()
	}
	c.jitter = func(max time.Duration) time.Duration { return max }
	return env
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, `{"error":{"code":"`+code+`","message":"`+msg+`","requestId":"req_123"}}`)
}
