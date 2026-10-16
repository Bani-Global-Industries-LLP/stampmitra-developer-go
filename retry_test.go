package stampmitra

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

const orderJSON = `{"externalOrderId":"o1","createdAt":"2026-10-15T09:41:07.000Z"}`

var newOrder = &CreateOrderParams{ExternalOrderID: "o1", ExternalCustomerID: "c1", ServiceCode: "estamp.x"}

// statusSeq answers with the given statuses in order (the last one repeats); 200 yields okBody.
func statusSeq(okBody string, statuses ...int) func(int, http.ResponseWriter, *http.Request) {
	return func(n int, w http.ResponseWriter, r *http.Request) {
		if n >= len(statuses) {
			n = len(statuses) - 1
		}
		if s := statuses[n]; s >= 200 && s < 300 {
			writeJSON(w, s, okBody)
			return
		}
		writeErr(w, statuses[n], "ERR", "boom")
	}
}

func TestGetRetriesThenSucceeds(t *testing.T) {
	env := newEnv(t, statusSeq(`[]`, 503, 503, 200))
	if _, err := env.c.Services.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(env.rec.all()); n != 3 {
		t.Fatalf("attempts = %d", n)
	}
	// jitter stub returns the ceiling: base 0.5s then 1s
	if len(env.sleeps) != 2 || env.sleeps[0] != 500*time.Millisecond || env.sleeps[1] != time.Second {
		t.Fatalf("sleeps = %v", env.sleeps)
	}
}

func TestRetryExhaustion(t *testing.T) {
	env := newEnv(t, statusSeq(`[]`, 503, 503, 503, 200))
	_, err := env.c.Services.List(context.Background())
	var api *APIError
	if !errors.As(err, &api) || api.Status != 503 || api.Attempts != 3 {
		t.Fatalf("got %v (%+v)", err, api)
	}
	if n := len(env.rec.all()); n != 3 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestMaxRetriesOverrides(t *testing.T) {
	env := newEnv(t, statusSeq(`[]`, 503, 200), WithMaxRetries(0))
	if _, err := env.c.Services.List(context.Background()); !IsUpstream(err) {
		t.Fatal(err)
	}
	if n := len(env.rec.all()); n != 1 {
		t.Fatalf("attempts = %d", n)
	}
	env = newEnv(t, statusSeq(`[]`, 503, 503, 503, 503, 503, 200))
	if _, err := env.c.Services.List(context.Background(), WithRequestMaxRetries(5)); err != nil {
		t.Fatal(err)
	}
	if n := len(env.rec.all()); n != 6 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestNoRetryOn4xx(t *testing.T) {
	for _, st := range []int{400, 401, 402, 403, 404, 409, 500} {
		env := newEnv(t, statusSeq(`[]`, st, 200))
		_, err := env.c.Services.List(context.Background())
		if err == nil {
			t.Fatalf("%d: expected error", st)
		}
		if n := len(env.rec.all()); n != 1 {
			t.Errorf("%d: attempts = %d", st, n)
		}
	}
}

func TestRetryableStatusesAreRetried(t *testing.T) {
	for _, st := range []int{429, 502, 503, 504} {
		env := newEnv(t, statusSeq(`[]`, st, 200))
		if _, err := env.c.Services.List(context.Background()); err != nil {
			t.Fatalf("%d: %v", st, err)
		}
		if n := len(env.rec.all()); n != 2 {
			t.Errorf("%d: attempts = %d", st, n)
		}
	}
}

func TestRetryAfterHonouredAndCapped(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			w.Header().Set("Retry-After", "2")
			writeErr(w, 429, "TOO_MANY_REQUESTS", "x")
			return
		}
		writeJSON(w, 200, `[]`)
	})
	if _, err := env.c.Services.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(env.sleeps) != 1 || env.sleeps[0] != 2*time.Second {
		t.Fatalf("sleeps = %v", env.sleeps)
	}

	env = newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			w.Header().Set("Retry-After", "86400")
			writeErr(w, 429, "TOO_MANY_REQUESTS", "x")
			return
		}
		writeJSON(w, 200, `[]`)
	})
	if _, err := env.c.Services.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if env.sleeps[0] != 60*time.Second {
		t.Fatalf("Retry-After must be capped at 60s, got %v", env.sleeps)
	}
}

func TestBackoffCeilingAndCap(t *testing.T) {
	c, _ := NewClient(testKey)
	for attempt, want := range []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 8 * time.Second, 8 * time.Second} {
		c.jitter = func(max time.Duration) time.Duration { return max }
		if got := c.retryDelay(attempt, 0); got != want {
			t.Errorf("attempt %d: ceiling %v, want %v", attempt, got, want)
		}
	}
	if got := c.retryDelay(1000, 0); got != 8*time.Second {
		t.Errorf("huge attempt: %v", got)
	}
	// full jitter really is random within [0, ceiling]
	c2, _ := NewClient(testKey)
	for i := 0; i < 500; i++ {
		d := c2.retryDelay(2, 0)
		if d < 0 || d > 2*time.Second {
			t.Fatalf("delay %v outside [0, 2s]", d)
		}
	}
	// and does not collapse to a constant
	distinct := map[time.Duration]bool{}
	for i := 0; i < 50; i++ {
		distinct[c2.retryDelay(3, 0)] = true
	}
	if len(distinct) < 10 {
		t.Errorf("jitter looks constant: %d distinct values", len(distinct))
	}
}

func TestPostAutoKeyReusedAcrossRetries(t *testing.T) {
	env := newEnv(t, statusSeq(orderJSON, 502, 503, 201))
	if _, err := env.c.Orders.Create(context.Background(), newOrder); err != nil {
		t.Fatal(err)
	}
	reqs := env.rec.all()
	if len(reqs) != 3 {
		t.Fatalf("attempts = %d", len(reqs))
	}
	k := reqs[0].Header.Get("Idempotency-Key")
	if len(k) != 36 {
		t.Fatalf("auto key = %q", k)
	}
	for _, r := range reqs {
		if r.Header.Get("Idempotency-Key") != k || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("key/content-type changed across retries: %v", r.Header)
		}
	}
	if reqs[0].Body != reqs[2].Body {
		t.Fatal("body changed across retries")
	}
}

func TestPostCallerKeyReused(t *testing.T) {
	env := newEnv(t, statusSeq(orderJSON, 503, 201))
	if _, err := env.c.Orders.Create(context.Background(), newOrder, WithIdempotencyKey("mine-1")); err != nil {
		t.Fatal(err)
	}
	for _, r := range env.rec.all() {
		if r.Header.Get("Idempotency-Key") != "mine-1" {
			t.Fatalf("key = %q", r.Header.Get("Idempotency-Key"))
		}
	}
}

func TestEveryAutoKeyPostCarriesFreshKeyAndIsRetried(t *testing.T) {
	ctx := context.Background()
	calls := map[string]func(c *Client) error{
		"verify.pan": func(c *Client) error {
			_, err := c.Verify.PAN(ctx, &PANVerifyParams{PAN: "ABCDE1234F"})
			return err
		},
		"duty": func(c *Client) error {
			_, err := c.Estamp.CalculateDuty(ctx, &EstampDutyParams{StateCode: "MH", ArticleCode: "5", StampDutyAmount: 100})
			return err
		},
		"parties": func(c *Client) error {
			_, err := c.Estamp.AddParties(ctx, "o1", &EstampPartiesParams{PartyName: "x"})
			return err
		},
		"intent": func(c *Client) error {
			_, err := c.Payments.Intents.Create(ctx, &CreatePaymentIntentParams{ExternalOrderID: "o1", AmountMinor: 100, Currency: "INR", PaymentConnectionID: "pc"})
			return err
		},
	}
	for name, fn := range calls {
		env := newEnv(t, statusSeq(`{"referenceId":"r","valid":true,"id":"x","externalOrderId":"o1","coreOrderId":null,"amountMinor":1,"currency":"INR","status":"s","providerIntentId":null,"createdAt":"t"}`, 503, 201))
		if err := fn(env.c); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		reqs := env.rec.all()
		if len(reqs) != 2 || reqs[0].Header.Get("Idempotency-Key") == "" || reqs[0].Header.Get("Idempotency-Key") != reqs[1].Header.Get("Idempotency-Key") {
			t.Fatalf("%s: %d attempts, keys %q/%q", name, len(reqs), reqs[0].Header.Get("Idempotency-Key"), reqs[len(reqs)-1].Header.Get("Idempotency-Key"))
		}
	}
	// two separate calls must not share a key
	env := newEnv(t, statusSeq(`{"referenceId":"r","valid":true}`, 200))
	_, _ = env.c.Verify.PAN(ctx, &PANVerifyParams{PAN: "ABCDE1234F"})
	_, _ = env.c.Verify.PAN(ctx, &PANVerifyParams{PAN: "ABCDE1234F"})
	r := env.rec.all()
	if r[0].Header.Get("Idempotency-Key") == r[1].Header.Get("Idempotency-Key") {
		t.Fatal("distinct calls shared an idempotency key")
	}
}

func TestReplayIsNotRetriedWithoutKey(t *testing.T) {
	env := newEnv(t, statusSeq(`{"id":"e","status":"QUEUED"}`, 503, 200))
	_, err := env.c.Webhooks.Events.Replay(context.Background(), "evt_1")
	if !IsUpstream(err) {
		t.Fatalf("got %v", err)
	}
	reqs := env.rec.all()
	if len(reqs) != 1 || reqs[0].Header.Get("Idempotency-Key") != "" {
		t.Fatalf("attempts %d, key %q", len(reqs), reqs[0].Header.Get("Idempotency-Key"))
	}
	// ... but is when the caller supplies a key
	env = newEnv(t, statusSeq(`{"id":"e","status":"QUEUED"}`, 503, 200))
	if _, err := env.c.Webhooks.Events.Replay(context.Background(), "evt_1", WithIdempotencyKey("k")); err != nil {
		t.Fatal(err)
	}
	if n := len(env.rec.all()); n != 2 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestPost4xxNotRetried(t *testing.T) {
	for _, st := range []int{400, 409} {
		env := newEnv(t, statusSeq(orderJSON, st, 201))
		if _, err := env.c.Orders.Create(context.Background(), newOrder); err == nil {
			t.Fatal("expected error")
		}
		if n := len(env.rec.all()); n != 1 {
			t.Errorf("%d: attempts = %d", st, n)
		}
	}
}

func TestConnectionRefusedIsRetriedForGetThenReported(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	c, err := NewClient(testKey, WithBaseURL("http://"+addr), WithMaxRetries(2))
	if err != nil {
		t.Fatal(err)
	}
	var sleeps []time.Duration
	c.sleep = func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return nil }
	_, err = c.Services.List(context.Background())
	var ce *ConnectionError
	if !errors.As(err, &ce) || ce.Timeout || !IsConnection(err) || IsTimeout(err) || ce.Attempts != 3 {
		t.Fatalf("got %v", err)
	}
	if len(sleeps) != 2 {
		t.Fatalf("sleeps = %v", sleeps)
	}
}

func TestConnectionDropRetried(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
			return
		}
		writeJSON(w, 200, `[]`)
	})
	if _, err := env.c.Services.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(env.rec.all()); n != 2 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestTimeoutRetriedForGetAndPostWithKey(t *testing.T) {
	slow := func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			<-r.Context().Done()
			return
		}
		writeJSON(w, 200, `[]`)
	}
	env := newEnv(t, slow, WithTimeout(100*time.Millisecond))
	if _, err := env.c.Services.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(env.rec.all()); n != 2 {
		t.Fatalf("attempts = %d", n)
	}

	env = newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			<-r.Context().Done()
			return
		}
		writeJSON(w, 201, orderJSON)
	}, WithTimeout(100*time.Millisecond))
	if _, err := env.c.Orders.Create(context.Background(), newOrder); err != nil {
		t.Fatal(err)
	}
	reqs := env.rec.all()
	if len(reqs) != 2 || reqs[0].Header.Get("Idempotency-Key") != reqs[1].Header.Get("Idempotency-Key") {
		t.Fatalf("POST retried after timeout must reuse the key: %v", reqs)
	}
}

func TestTimeoutExhausted(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }, WithTimeout(50*time.Millisecond))
	_, err := env.c.Services.List(context.Background())
	if !IsTimeout(err) {
		t.Fatalf("got %v", err)
	}
	if n := len(env.rec.all()); n != 3 {
		t.Fatalf("attempts = %d", n)
	}
}

func TestCancelDuringBackoffStopsRetrying(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	env := newEnv(t, statusSeq(`[]`, 503, 200))
	env.c.sleep = func(c context.Context, d time.Duration) error { cancel(); return c.Err() }
	_, err := env.c.Services.List(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if n := len(env.rec.all()); n != 1 {
		t.Fatalf("attempts = %d", n)
	}
}
