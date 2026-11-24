package stampmitra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const estampOrderBody = `{"id":"estord_1","orderId":"o-1","status":"PENDING_REVIEW","stamps":[{"id":"eststamp_1"}]}`

// ---- retry scenarios replayed with exact headers and bodies --------------------------------------

// TestRetryScenariosReplay replays every scenario of the vendored retry-scenarios.json against an
// httptest server that scripts each response, including Retry-After headers and error bodies, which
// the spec repo's mock cannot. It checks attempts, outcome, key reuse and the delay before attempt 2.
func TestRetryScenariosReplay(t *testing.T) {
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
	if len(f.Scenarios) != 54 {
		t.Fatalf("%d scenarios, want 54", len(f.Scenarios))
	}
	for _, s := range f.Scenarios {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			var mu sync.Mutex
			var reqs []http.Header
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				mu.Lock()
				n := len(reqs)
				reqs = append(reqs, r.Header.Clone())
				mu.Unlock()
				if n >= len(s.Sequence) {
					n = len(s.Sequence) - 1
				}
				status, hdrs, body := 0, map[string]any{}, any(nil)
				switch v := s.Sequence[n].(type) {
				case string:
					if v == "timeout" {
						select {
						case <-r.Context().Done():
						case <-time.After(2 * time.Second):
						}
						return
					}
					hj, _ := w.(http.Hijacker)
					conn, _, _ := hj.Hijack()
					_ = conn.Close()
					return
				case float64:
					status = int(v)
				case map[string]any:
					status = int(v["status"].(float64))
					if h, ok := v["headers"].(map[string]any); ok {
						hdrs = h
					}
					body = v["body"]
				}
				for k, v := range hdrs {
					w.Header().Set(k, fmt.Sprint(v))
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(status)
				switch {
				case body != nil:
					_ = json.NewEncoder(w).Encode(body)
				case status >= 400:
					code := "ERR"
					if m := regexp.MustCompile(`\(([A-Z_]{4,})\)`).FindStringSubmatch(fmt.Sprint(s.Outcome)); m != nil {
						code = m[1]
					}
					_, _ = fmt.Fprintf(w, `{"error":{"code":%q,"message":"x","requestId":"req_1"}}`, code)
				default:
					_, _ = io.WriteString(w, `{}`)
				}
			}))
			defer srv.Close()
			tr := &http.Transport{DisableKeepAlives: true}
			defer tr.CloseIdleConnections()
			maxRetries := 2
			if s.MaxRetries != nil {
				maxRetries = *s.MaxRetries
			}
			c, err := NewClient(testKey, WithBaseURL(srv.URL), WithHTTPClient(&http.Client{Transport: tr}), WithMaxRetries(maxRetries))
			if err != nil {
				t.Fatal(err)
			}
			var sleeps []time.Duration
			c.sleep = func(ctx context.Context, d time.Duration) error { sleeps = append(sleeps, d); return ctx.Err() }
			c.jitter = func(max time.Duration) time.Duration { return max }

			pol := scenarioPolicy(s)
			var opts []RequestOption
			if string(s.IdempotencyKey) == "true" {
				opts = append(opts, WithIdempotencyKey("caller-key-1"))
			}
			if hasTimeout(s) {
				opts = append(opts, WithRequestTimeout(100*time.Millisecond))
			}
			var body any
			if s.Method == "POST" {
				body = map[string]any{}
			}
			var out GenericResponse
			err = c.doWith(context.Background(), s.Method, s.Path, nil, body, &out, s.Method == "POST", pol, opts)

			wantAttempts, wantOutcome := s.ExpectedAttempts, s.Outcome
			if s.Method == "POST" && pol == retryDefault && string(s.IdempotencyKey) == "false" {
				// This SDK sends an automatic Idempotency-Key on every such POST, so it is retried like a keyed one.
				wantAttempts, wantOutcome = simulate(seqItems(s), maxRetries)
			}
			if o, ok := wantOutcome.(float64); ok && o >= 200 && o < 300 {
				wantOutcome = 200
			}
			checkOutcome(t, err, wantOutcome)
			mu.Lock()
			defer mu.Unlock()
			if len(reqs) != wantAttempts {
				t.Fatalf("attempts = %d, want %d (%s)", len(reqs), wantAttempts, s.Name)
			}
			key := reqs[0].Get("Idempotency-Key")
			switch {
			case pol == retryNever && key != "":
				t.Errorf("a never-retried operation sent an Idempotency-Key")
			case pol == retryDefault && s.Method == "POST" && key == "":
				t.Errorf("POST without an Idempotency-Key")
			case s.Method == "GET" && key != "":
				t.Errorf("GET sent an Idempotency-Key")
			}
			for _, h := range reqs {
				if h.Get("Idempotency-Key") != key {
					t.Fatal("Idempotency-Key changed across attempts")
				}
			}
			if s.ExpectedDelay != nil && wantAttempts > 1 {
				if want := time.Duration(*s.ExpectedDelay * float64(time.Second)); len(sleeps) == 0 || sleeps[0] != want {
					t.Errorf("delay before attempt 2 = %v, want %v", sleeps, want)
				}
			}
		})
	}
}

func hasTimeout(s retryScenario) bool {
	for _, it := range s.Sequence {
		if v, ok := it.(string); ok && v == "timeout" {
			return true
		}
	}
	return false
}

func seqItems(s retryScenario) []string {
	var items []string
	for _, it := range s.Sequence {
		switch v := it.(type) {
		case string:
			items = append(items, "conn") // a timeout is retried like a dropped connection
		case float64:
			items = append(items, fmt.Sprint(int(v)))
		case map[string]any:
			items = append(items, fmt.Sprint(int(v["status"].(float64))))
		}
	}
	return items
}

// ---- idempotency key, retries, Retry-After, never-retry ------------------------------------------

func TestCreateOrderAutoGeneratesAndReusesKey(t *testing.T) {
	env := newEnv(t, statusSeq(estampOrderBody, 503, 502, 201))
	o, err := env.c.Estamp.Orders.Create(context.Background(), testOrderParams)
	if err != nil || o.ID != "estord_1" || o.Stamps[0].ID != "eststamp_1" {
		t.Fatalf("%v %+v", err, o)
	}
	reqs := env.rec.all()
	if len(reqs) != 3 {
		t.Fatalf("attempts = %d", len(reqs))
	}
	uuid := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	k := reqs[0].Header.Get("Idempotency-Key")
	if !uuid.MatchString(k) {
		t.Fatalf("key = %q", k)
	}
	for _, r := range reqs {
		if r.Header.Get("Idempotency-Key") != k || r.Body != reqs[0].Body {
			t.Fatal("key or body changed across retries")
		}
	}
	// a second call gets a different key; a caller key wins
	_, _ = env.c.Estamp.Orders.Create(context.Background(), testOrderParams)
	if env.rec.all()[3].Header.Get("Idempotency-Key") == k {
		t.Fatal("two orders shared a key")
	}
	_, _ = env.c.Estamp.Orders.Create(context.Background(), testOrderParams, WithIdempotencyKey("mine"))
	if got := env.rec.all()[4].Header.Get("Idempotency-Key"); got != "mine" {
		t.Fatalf("key = %q", got)
	}
}

func TestRetryAfterHonouredOn429503And409InProgress(t *testing.T) {
	cases := []struct {
		name   string
		status int
		code   string
		retry  bool
	}{
		{"429", 429, "TOO_MANY_REQUESTS", true},
		{"503", 503, "UNAVAILABLE", true},
		{"409 in progress", 409, "IDEMPOTENCY_IN_PROGRESS", true},
		{"409 other conflict", 409, "CONFLICT", false},
		{"422 key reused", 422, "IDEMPOTENCY_KEY_REUSED", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
				if n == 0 {
					w.Header().Set("Retry-After", "7")
					writeErr(w, tc.status, tc.code, "x")
					return
				}
				writeJSON(w, 201, estampOrderBody)
			})
			_, err := env.c.Estamp.Orders.Create(context.Background(), testOrderParams)
			if tc.retry {
				if err != nil || len(env.sleeps) != 1 || env.sleeps[0] != 7*time.Second {
					t.Fatalf("err %v sleeps %v", err, env.sleeps)
				}
				return
			}
			if err == nil || len(env.rec.all()) != 1 {
				t.Fatalf("err %v attempts %d", err, len(env.rec.all()))
			}
		})
	}
	// GET honours Retry-After on 503 too, and 409 in progress is not retried without a key
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if n == 0 {
			w.Header().Set("Retry-After", "2")
			writeErr(w, 503, "UNAVAILABLE", "x")
			return
		}
		writeJSON(w, 200, `{"code":"MH","name":"Maharashtra"}`)
	})
	if _, err := env.c.Estamp.GetState(context.Background(), "MH"); err != nil || env.sleeps[0] != 2*time.Second {
		t.Fatalf("%v %v", err, env.sleeps)
	}
	env = newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeErr(w, 409, "IDEMPOTENCY_IN_PROGRESS", "x") })
	if _, err := env.c.Estamp.Orders.Cancel(context.Background(), "estord_1"); !IsConflict(err) || len(env.rec.all()) != 1 {
		t.Fatalf("%v", err)
	}
}

// TestNeverRetrySet: order cancel, stamp cancel, replace, PAN-card link and both wallet top-ups are
// sent exactly once, for every retryable failure, with or without a caller key, and carry no key.
func TestNeverRetrySet(t *testing.T) {
	calls := map[string]func(c *Client, o ...RequestOption) error{
		"order cancel": func(c *Client, o ...RequestOption) error {
			_, err := c.Estamp.Orders.Cancel(context.Background(), "estord_1", o...)
			return err
		},
		"stamp cancel": func(c *Client, o ...RequestOption) error {
			_, err := c.Estamp.Stamps.Cancel(context.Background(), "eststamp_1", o...)
			return err
		},
		"stamp replace": func(c *Client, o ...RequestOption) error {
			_, err := c.Estamp.Stamps.Replace(context.Background(), "eststamp_1", o...)
			return err
		},
		"pan-card link": func(c *Client, o ...RequestOption) error {
			_, err := c.Estamp.Stamps.SetPANCard(context.Background(), "eststamp_1", &EstampPANCardParams{PANCardImage: "https://x.example/p.jpg"}, o...)
			return err
		},
		"stamp duty top-up": func(c *Client, o ...RequestOption) error {
			_, err := c.StampDutyWallet.Topup(context.Background(), &WalletTopupParams{BaseAmount: 1000}, o...)
			return err
		},
		"service fee top-up": func(c *Client, o ...RequestOption) error {
			_, err := c.ServiceFeeWallet.Topup(context.Background(), &WalletTopupParams{BaseAmount: 500}, o...)
			return err
		},
		"webhook replay": func(c *Client, o ...RequestOption) error {
			_, err := c.Webhooks.Events.Replay(context.Background(), "evt_1", o...)
			return err
		},
	}
	for name, call := range calls {
		for _, st := range []int{429, 502, 503, 504} {
			for _, withKey := range []bool{false, true} {
				env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Retry-After", "1")
					writeErr(w, st, "ERR", "x")
				})
				var opts []RequestOption
				if withKey {
					opts = append(opts, WithIdempotencyKey("k"))
				}
				var api *APIError
				if err := call(env.c, opts...); !errors.As(err, &api) || api.Status != st {
					t.Fatalf("%s %d: %v", name, st, err)
				}
				reqs := env.rec.all()
				if len(reqs) != 1 || reqs[0].Header.Get("Idempotency-Key") != "" {
					t.Errorf("%s %d key=%v: %d attempts, key %q", name, st, withKey, len(reqs), reqs[0].Header.Get("Idempotency-Key"))
				}
			}
		}
		// a dropped connection is not retried either
		env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		})
		if err := call(env.c); !IsConnection(err) || len(env.rec.all()) != 1 {
			t.Errorf("%s: connection drop: %v, %d attempts", name, err, len(env.rec.all()))
		}
	}
}

func TestValidateIsRetriedWithoutKey(t *testing.T) {
	env := newEnv(t, statusSeq(`{"valid":true,"status":"READY_TO_PLACE","errors":[],"warnings":[]}`, 503, 201))
	res, err := env.c.Estamp.Orders.Validate(context.Background(), testOrderParams)
	if err != nil || !res.Valid || res.Status != "READY_TO_PLACE" {
		t.Fatalf("%v %+v", err, res)
	}
	reqs := env.rec.all()
	if len(reqs) != 2 || reqs[0].Header.Get("Idempotency-Key") != "" {
		t.Fatalf("attempts %d, key %q", len(reqs), reqs[0].Header.Get("Idempotency-Key"))
	}
}

// ---- multipart upload --------------------------------------------------------------------------

type uploadSeen struct {
	ctype, key, fileName, fileType, typePart string
	file                                     []byte
}

func parseUpload(t *testing.T, r *http.Request, body string) uploadSeen {
	t.Helper()
	mt, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "multipart/form-data" {
		t.Fatalf("content type %q", r.Header.Get("Content-Type"))
	}
	u := uploadSeen{ctype: mt, key: r.Header.Get("Idempotency-Key")}
	mr := multipart.NewReader(strings.NewReader(body), params["boundary"])
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			return u
		}
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(p)
		switch p.FormName() {
		case "file":
			u.file, u.fileName, u.fileType = b, p.FileName(), p.Header.Get("Content-Type")
		case "type":
			u.typePart = string(b)
		default:
			t.Fatalf("unexpected part %q", p.FormName())
		}
	}
}

const uploadReply = `{"documentId":"estdoc_1","orderId":"estord_1","type":"pan_card","fileName":"pan.pdf","mimeType":"application/pdf","sizeBytes":5,"receivedAt":"2026-10-29T09:12:03.000Z","status":"RECEIVED"}`

func TestUploadDocumentMultipart(t *testing.T) {
	env := newEnv(t, statusSeq(uploadReply, 502, 201))
	doc, err := env.c.Estamp.Orders.UploadDocument(context.Background(), "estord_1", &EstampDocumentUploadParams{
		File: strings.NewReader(testPDF), FileName: "pan.pdf", Type: DocumentTypePANCard,
	})
	if err != nil || doc.Status != "RECEIVED" || doc.DocumentID != "estdoc_1" {
		t.Fatalf("%v %+v", err, doc)
	}
	reqs := env.rec.all()
	if len(reqs) != 2 || reqs[0].Path != "/v1/estamp/orders/estord_1/documents" {
		t.Fatalf("%+v", reqs)
	}
	var first uploadSeen
	for i, r := range reqs {
		u := parseUpload(t, &http.Request{Header: r.Header}, r.Body)
		if u.fileName != "pan.pdf" || u.fileType != "application/pdf" || u.typePart != "pan_card" || string(u.file) != testPDF {
			t.Errorf("attempt %d: %+v", i, u)
		}
		if u.key == "" {
			t.Error("missing auto Idempotency-Key")
		}
		if i == 0 {
			first = u
		} else if u.key != first.key {
			t.Error("key changed across the retry")
		}
	}
}

func TestUploadDocumentOptionsAndValidation(t *testing.T) {
	env := newEnv(t, statusSeq(uploadReply, 201))
	// optional type part omitted; content type detected from the bytes when the name has no known extension
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("x", 20)
	if _, err := env.c.Estamp.Orders.UploadDocument(context.Background(), "estord_1", &EstampDocumentUploadParams{File: strings.NewReader(png)}, WithIdempotencyKey("mine")); err != nil {
		t.Fatal(err)
	}
	r := env.rec.all()[0]
	u := parseUpload(t, &http.Request{Header: r.Header}, r.Body)
	if u.typePart != "" || u.fileType != "image/png" || u.key != "mine" {
		t.Errorf("%+v", u)
	}
	for name, p := range map[string]*EstampDocumentUploadParams{
		"nil":       nil,
		"no reader": {},
		"empty":     {File: strings.NewReader("")},
		"too large": {File: strings.NewReader(strings.Repeat("a", 10<<20+1)), ContentType: "application/pdf"},
		"bad type":  {File: strings.NewReader("hello"), FileName: "a.txt"},
		"bad kind":  {File: strings.NewReader(testPDF), Type: "aadhaar"},
	} {
		if _, err := env.c.Estamp.Orders.UploadDocument(context.Background(), "estord_1", p); !IsValidation(err) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(env.rec.all()) != 1 {
		t.Error("an invalid upload reached the network")
	}
	if _, err := env.c.Estamp.Orders.UploadDocument(context.Background(), "a/b", &EstampDocumentUploadParams{File: strings.NewReader(testPDF)}); !IsValidation(err) {
		t.Error("path id not validated")
	}
}

func TestUploadBodyIsNeverLogged(t *testing.T) {
	var logs strings.Builder
	env := newEnv(t, statusSeq(uploadReply, 201), WithLogger(debugLogger(&logs)))
	_, err := env.c.Estamp.Orders.UploadDocument(context.Background(), "estord_1", &EstampDocumentUploadParams{File: strings.NewReader("%PDF-SECRETCONTENT"), FileName: "secret-name.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), "SECRETCONTENT") || strings.Contains(logs.String(), "secret-name") || !strings.Contains(logs.String(), "multipart/form-data body") {
		t.Errorf("log: %s", logs.String())
	}
}

// ---- types -------------------------------------------------------------------------------------

func TestPartyUnion(t *testing.T) {
	line := EstampStampLine{State: "MH", ArticleCode: "A", Denomination: 100, Quantity: 2,
		FirstParty:   PartyDetails(EstampParty{Name: "Jane", Address: "Pune", Gender: GenderFemale}),
		SecondParty:  ptr(PartyName("John")),
		PropertyArea: &EstampPropertyArea{Unit: AreaSquareFeet, Value: 850}}
	b, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"state":"MH","articleCode":"A","denomination":100,"firstParty":{"name":"Jane","address":"Pune","gender":"FEMALE"},"secondParty":"John","propertyArea":{"unit":"Square Feet","value":850},"quantity":2}`
	if string(b) != want {
		t.Fatalf("got  %s\nwant %s", b, want)
	}
	var back EstampStampLine
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	d, ok := back.FirstParty.Details()
	if !ok || d.Address != "Pune" || back.FirstParty.Name() != "Jane" {
		t.Errorf("first party %+v", back.FirstParty)
	}
	if _, ok := back.SecondParty.Details(); ok || back.SecondParty.Name() != "John" {
		t.Errorf("second party %+v", back.SecondParty)
	}
	// no second party: omitted; arrays are refused
	b, _ = json.Marshal(EstampStampLine{FirstParty: PartyName("A")})
	if strings.Contains(string(b), "secondParty") {
		t.Errorf("%s", b)
	}
	if err := json.Unmarshal([]byte(`["a"]`), &Party{}); err == nil {
		t.Error("array accepted")
	}
}

func TestOrderParamsValidation(t *testing.T) {
	ok := func() EstampStampLine {
		return EstampStampLine{State: "MH", ArticleCode: "A", Denomination: 100, FirstParty: PartyName("A")}
	}
	many := make([]EstampStampLine, 5001)
	for i := range many {
		many[i] = ok()
	}
	other := ok()
	other.State = "KA"
	big := ok()
	big.Quantity = 1001
	zero := ok()
	zero.Denomination = 0
	noparty := ok()
	noparty.FirstParty = Party{}
	http1 := ok()
	http1.PANCardImage = "http://x.example/p.jpg"
	bad := map[string][]EstampStampLine{"none": nil, "5001 lines": many, "mixed state": {ok(), other}, "quantity 1001": {big}, "zero duty": {zero}, "no first party": {noparty}, "http pan link": {http1}}
	env := newEnv(t, statusSeq(estampOrderBody, 201))
	for name, st := range bad {
		if _, err := env.c.Estamp.Orders.Create(context.Background(), &EstampOrderParams{Stamps: st}); !IsValidation(err) {
			t.Errorf("%s: %v", name, err)
		}
		if _, err := env.c.Estamp.Orders.Validate(context.Background(), &EstampOrderParams{Stamps: st}); !IsValidation(err) {
			t.Errorf("validate %s: %v", name, err)
		}
	}
	max := ok()
	max.Quantity = 1000
	if _, err := env.c.Estamp.Orders.Create(context.Background(), &EstampOrderParams{Stamps: []EstampStampLine{max}}); err != nil {
		t.Errorf("quantity 1000: %v", err)
	}
	if n := len(env.rec.all()); n != 1 {
		t.Errorf("%d requests", n)
	}
	for _, f := range []func() error{
		func() error {
			_, e := env.c.StampDutyWallet.Topup(context.Background(), &WalletTopupParams{})
			return e
		},
		func() error {
			_, e := env.c.ServiceFeeWallet.Topup(context.Background(), &WalletTopupParams{BaseAmount: -1})
			return e
		},
		func() error {
			_, e := env.c.Estamp.Stamps.SetPANCard(context.Background(), "s", &EstampPANCardParams{PANCardImage: "http://x"})
			return e
		},
		func() error {
			_, e := env.c.Estamp.GetStateRequirements(context.Background(), "MH", &StateRequirementsParams{Category: "x"})
			return e
		},
		func() error {
			_, e := env.c.Estamp.Orders.List(context.Background(), &EstampOrdersListParams{Limit: 101})
			return e
		},
		func() error { _, e := env.c.Estamp.Orders.Get(context.Background(), ""); return e },
	} {
		if err := f(); !IsValidation(err) {
			t.Errorf("%v", err)
		}
	}
}

func TestQueryParameters(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, `{}`) })
	ctx := context.Background()
	_, _ = env.c.Estamp.GetStatesAvailability(ctx, &StatesAvailabilityParams{ActiveOnly: true})
	_, _ = env.c.Estamp.GetStatesAvailability(ctx, nil)
	_, _ = env.c.Estamp.GetStateRequirements(ctx, "MH", &StateRequirementsParams{ArticleCode: "ART1", Denomination: 1000.5, Category: CategoryTraditional})
	_, _ = env.c.Estamp.Orders.List(ctx, &EstampOrdersListParams{Page: 2, Limit: 50, Cursor: "c1"})
	_, _ = env.c.StampDutyWallet.ListInvoices(ctx, &WalletListParams{Limit: 5, Cursor: "c2"})
	want := []string{"active=true", "", "articleCode=ART1&category=traditional_stamp&denomination=1000.5", "cursor=c1&limit=50&page=2", "cursor=c2&limit=5"}
	for i, r := range env.rec.all() {
		if r.Query != want[i] {
			t.Errorf("query %d = %q, want %q", i, r.Query, want[i])
		}
	}
}

// ---- errors --------------------------------------------------------------------------------------

func TestAPIErrorDetails(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/estamp/orders" {
			writeJSON(w, 400, `{"error":{"code":"VALIDATION_FAILED","message":"VALIDATION_FAILED: 2 findings","requestId":"req_9","details":{"errors":[{"field":"stamps[0].firstParty.name","code":"REQUIRED","message":"Required","index":0},{"field":"","code":"TOO_MANY","message":"Too many"}]}}}`)
			return
		}
		writeJSON(w, 402, `{"error":{"code":"PAYMENT_REQUIRED","message":"short","requestId":"req_10","details":{"wallet":"SERVICE_FEE","other":1}}}`)
	})
	_, err := env.c.Estamp.Orders.Create(context.Background(), testOrderParams)
	var api *APIError
	if !errors.As(err, &api) || !IsValidation(err) || api.Code != "VALIDATION_FAILED" || api.RequestID != "req_9" || len(api.FieldErrors) != 2 {
		t.Fatalf("%v %+v", err, api)
	}
	if f := api.FieldErrors[0]; f.Field != "stamps[0].firstParty.name" || f.Code != "REQUIRED" || f.Index == nil || *f.Index != 0 || api.FieldErrors[1].Index != nil {
		t.Errorf("%+v", api.FieldErrors)
	}
	_, err = env.c.StampDutyWallet.GetBalance(context.Background())
	if !errors.As(err, &api) || !IsPaymentRequired(err) || api.Wallet != WalletServiceFee || !strings.Contains(string(api.Details), `"other"`) {
		t.Fatalf("%v %+v", err, api)
	}
	// 422 is a validation error; an odd details shape never costs the code
	env = newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 422, `{"error":{"code":"IDEMPOTENCY_KEY_REUSED","message":"m","requestId":"r","details":"oops"}}`)
	})
	_, err = env.c.Estamp.Orders.Create(context.Background(), testOrderParams)
	if !errors.As(err, &api) || !IsValidation(err) || api.Code != "IDEMPOTENCY_KEY_REUSED" || len(api.FieldErrors) != 0 {
		t.Fatalf("%v %+v", err, api)
	}
}

// ---- money, lists, iterators ----------------------------------------------------------------------

func TestTopupAmountTypesAndNoFeeMath(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "stamp-duty") {
			writeJSON(w, 200, `{"baseAmount":1000,"processingFeePercent":"6.00","processingFeeAmount":"60.00","gstAmount":10.8,"totalCharged":1070.8,"creditAmount":"1000.00","topupId":"cmw1","razorpayOrderId":"order_1"}`)
			return
		}
		writeJSON(w, 200, `{"baseAmount":"500.00","gstAmount":"90.00","processingFeePercent":"0.00","processingFeeAmount":"0.00","creditAmount":"500.00","totalCharged":"590.00"}`)
	})
	sd, err := env.c.StampDutyWallet.Topup(context.Background(), &WalletTopupParams{BaseAmount: 1000})
	if err != nil || sd.BaseAmount != 1000 || sd.GstAmount != 10.8 || sd.TotalCharged != 1070.8 || sd.ProcessingFeeAmount != "60.00" || sd.ProcessingFeePercent != "6.00" || sd.CreditAmount != "1000.00" || sd.TopupID != "cmw1" {
		t.Fatalf("%v %+v", err, sd)
	}
	sf, err := env.c.ServiceFeeWallet.Topup(context.Background(), &WalletTopupParams{BaseAmount: 500})
	if err != nil || sf.TotalCharged != "590.00" || sf.GstAmount != "90.00" {
		t.Fatalf("%v %+v", err, sf)
	}
	if b := env.rec.all()[0].Body; b != `{"baseAmount":1000}` {
		t.Errorf("body %s", b)
	}
	// topupId is optional (live only)
	var nt StampDutyWalletTopup
	if err := json.Unmarshal([]byte(`{"baseAmount":1,"processingFeePercent":"6.00","processingFeeAmount":"0.06","gstAmount":0.01,"totalCharged":1.07,"creditAmount":"1.00","sandbox":true}`), &nt); err != nil || nt.TopupID != "" {
		t.Fatalf("%v %+v", err, nt)
	}
}

func TestDistrictsAndSROsAreBareArrays(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/sros") {
			writeJSON(w, 200, `[]`)
			return
		}
		writeJSON(w, 200, `[{"code":"MUM","name":"Mumbai"}]`)
	})
	d, err := env.c.Estamp.ListDistricts(context.Background(), "MH")
	if err != nil || len(d) != 1 || d[0].Code != "MUM" {
		t.Fatalf("%v %+v", err, d)
	}
	s, err := env.c.Estamp.ListSROs(context.Background(), "MH", "MUM")
	if err != nil || s == nil || len(s) != 0 {
		t.Fatalf("%v %#v", err, s)
	}
}

func TestOrderAndWalletIterators(t *testing.T) {
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		isOrders := r.URL.Path == "/v1/estamp/orders"
		item := func(id string) string {
			if isOrders {
				return `{"id":"` + id + `","status":"PENDING_REVIEW"}`
			}
			return `{"id":"` + id + `","type":"TOPUP","amount":"1.00","balanceAfter":"1.00","referenceId":"r","description":"d","mode":"SANDBOX","createdAt":"2026-10-29T09:12:03.000Z"}`
		}
		switch r.URL.Query().Get("cursor") {
		case "":
			writeJSON(w, 200, `{"items":[`+item("a")+`,`+item("b")+`],"nextCursor":"c1"}`)
		case "c1":
			writeJSON(w, 200, `{"items":[],"nextCursor":"c2"}`)
		default:
			writeJSON(w, 200, `{"items":[`+item("c")+`],"nextCursor":null}`)
		}
	})
	var ids []string
	it := env.c.Estamp.Orders.ListAll(&EstampOrdersListParams{Limit: 2, Page: 3})
	for it.Next(context.Background()) {
		ids = append(ids, it.Item().ID)
	}
	if it.Err() != nil || strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("%v %v", ids, it.Err())
	}
	if q := env.rec.all()[0].Query; q != "limit=2" {
		t.Errorf("page must not be sent with an iterator: %q", q)
	}
	ids = nil
	wit := env.c.ServiceFeeWallet.ListAllTransactions(nil)
	for wit.Next(context.Background()) {
		ids = append(ids, wit.Item().ID)
	}
	if wit.Err() != nil || strings.Join(ids, ",") != "a,b,c" {
		t.Fatalf("%v %v", ids, wit.Err())
	}
	// a cursor that does not advance is an error, not an endless loop
	env = newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, `{"items":[],"nextCursor":"same"}`)
	})
	sit := env.c.StampDutyWallet.ListAllTransactions(&WalletListParams{Cursor: "same"})
	if sit.Next(context.Background()) || !errors.Is(sit.Err(), ErrDecode) {
		t.Errorf("stuck cursor: %v", sit.Err())
	}
}

func TestVersionAndUserAgent(t *testing.T) {
	if Version != "0.2.0" {
		t.Fatalf("Version = %q", Version)
	}
	env := newEnv(t, func(n int, w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, `{}`) })
	_, _ = env.c.Account.GetBalance(context.Background())
	if ua := env.rec.all()[0].Header.Get("User-Agent"); !strings.HasPrefix(ua, "stampmitra-go/0.2.0 ") {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestRedactMasksOrderPersonalData(t *testing.T) {
	out := RedactBody([]byte(`{"stamps":[{"firstParty":{"name":"Jane","address":"x","fatherHusbandName":"Sam","phone":"9876543210"},"secondParty":"John","panCardImage":"https://x/p.jpg","payerPhone":"9876543210","denomination":100}]}`))
	for _, s := range []string{"Jane", "Sam", "John", "9876543210", "x/p.jpg"} {
		if strings.Contains(out, s) {
			t.Errorf("%q leaked: %s", s, out)
		}
	}
	if !strings.Contains(out, `"denomination":100`) {
		t.Errorf("non-personal data lost: %s", out)
	}
}

func debugLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestUploadFileNameIsQuotedAndSanitised(t *testing.T) {
	env := newEnv(t, statusSeq(uploadReply, 201))
	_, err := env.c.Estamp.Orders.UploadDocument(context.Background(), "estord_1", &EstampDocumentUploadParams{File: strings.NewReader(testPDF), FileName: "dir/we\"ird\r\nname.pdf"})
	if err != nil {
		t.Fatal(err)
	}
	r := env.rec.all()[0]
	if !strings.Contains(r.Body, `filename="we\"irdname.pdf"`) || strings.Contains(r.Body, "dir/") {
		t.Errorf("body: %s", r.Body)
	}
}
