package stampmitra_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	stampmitra "github.com/Bani-Global-Industries-LLP/stampmitra-developer-go"
)

// api starts a throwaway server standing in for the StampMitra API and returns a client for it.
func api(h http.HandlerFunc, opts ...stampmitra.Option) (*stampmitra.Client, func()) {
	srv := httptest.NewServer(h)
	c, err := stampmitra.NewClient("sk_test_exampleKey1234", append([]stampmitra.Option{stampmitra.WithBaseURL(srv.URL)}, opts...)...)
	if err != nil {
		panic(err)
	}
	return c, srv.Close
}

func respond(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}
}

func Example() {
	c, closeAPI := api(respond(200, `{"referenceId":"SMV-20261016-8F42K7","pan":"ABCDE1234F","valid":true,"nameMatchResult":"EXACT_MATCH"}`))
	defer closeAPI()

	// In real code: client, err := stampmitra.NewClient("") // reads STAMPMITRA_API_KEY
	pan, err := c.Verify.PAN(context.Background(), &stampmitra.PANVerifyParams{PAN: "abcde1234f", Name: "Rahul Kumar"})
	if err != nil {
		panic(err)
	}
	fmt.Println(pan.Valid, pan.NameMatchResult, pan.ReferenceID)
	// Output: true EXACT_MATCH SMV-20261016-8F42K7
}

func ExampleNewClient() {
	_, err := stampmitra.NewClient("not-a-key")
	fmt.Println(errors.Is(err, stampmitra.ErrConfig))

	client, err := stampmitra.NewClient("sk_live_EXAMPLEKEY00000000",
		stampmitra.WithTimeout(10*time.Second),
		stampmitra.WithMaxRetries(3),
		stampmitra.WithAppInfo(stampmitra.AppInfo{Name: "my-service", Version: "1.4.0"}),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(client.Environment)
	fmt.Println(client) // the key is never printed
	// Output:
	// true
	// live
	// stampmitra.Client{environment: live, baseURL: https://api-b2b.stampmitra.in, apiKey: [REDACTED]}
}

func ExampleOrdersService_Create() {
	c, closeAPI := api(respond(201, `{"externalOrderId":"ord_ext_123","createdAt":"2026-10-15T09:41:07.000Z","serviceCode":"estamp.instrument.rent_agreement"}`))
	defer closeAPI()
	ctx := context.Background()

	// The SDK adds an Idempotency-Key automatically; pass your own to make a retry of YOUR call safe too.
	order, err := c.Orders.Create(ctx, &stampmitra.CreateOrderParams{
		ExternalOrderID:    "ord_ext_123",
		ExternalCustomerID: "cust_ext_456",
		ServiceCode:        "estamp.instrument.rent_agreement",
		Details:            map[string]any{"state": "MH", "stampValue": 500},
	}, stampmitra.WithIdempotencyKey("order-ord_ext_123"))
	if err != nil {
		panic(err)
	}
	// Orders are addressed by YOUR externalOrderId.
	fmt.Println(order.ExternalOrderID)
	// Output: ord_ext_123
}

func ExampleLogsService_ListAll() {
	pages := map[string]string{
		"":   `{"items":[{"id":"log-1","createdAt":"2026-10-15T09:41:07.000Z","actorType":"api_key","service":"pan.verify"}],"nextCursor":"c1"}`,
		"c1": `{"items":[{"id":"log-2","createdAt":"2026-10-15T09:42:07.000Z","actorType":"api_key","service":"gstin.verify"}],"nextCursor":null}`,
	}
	c, closeAPI := api(func(w http.ResponseWriter, r *http.Request) {
		respond(200, pages[r.URL.Query().Get("cursor")])(w, r)
	})
	defer closeAPI()
	ctx := context.Background()

	it := c.Logs.ListAll(&stampmitra.LogsListParams{Limit: 1, Status: "success"})
	for it.Next(ctx) {
		fmt.Println(it.Log().ID, *it.Log().Service)
	}
	if err := it.Err(); err != nil {
		panic(err)
	}
	// Output:
	// log-1 pan.verify
	// log-2 gstin.verify
}

func ExampleAPIError() {
	c, closeAPI := api(respond(404, `{"error":{"code":"NOT_FOUND","message":"Order not found","requestId":"req_42"}}`), stampmitra.WithMaxRetries(0))
	defer closeAPI()

	_, err := c.Orders.Get(context.Background(), "missing")

	var apiErr *stampmitra.APIError
	if errors.As(err, &apiErr) {
		fmt.Println(apiErr.Status, apiErr.Code, apiErr.RequestID)
	}
	fmt.Println(stampmitra.IsNotFound(err), errors.Is(err, stampmitra.ErrNotFound), stampmitra.IsRateLimit(err))
	// Output:
	// 404 NOT_FOUND req_42
	// true true false
}

func ExampleValidationError() {
	c, closeAPI := api(func(http.ResponseWriter, *http.Request) { panic("never called: validation happens before the network") })
	defer closeAPI()

	_, err := c.Verify.PAN(context.Background(), &stampmitra.PANVerifyParams{PAN: "ABCDE-1234F"})
	fmt.Println(errors.Is(err, stampmitra.ErrValidation))
	fmt.Println(err)
	// Output:
	// true
	// stampmitra: invalid pan: must be 5 letters, 4 digits, 1 letter
}

func ExampleWithRawResponse() {
	c, closeAPI := api(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", "req_77")
		respond(200, `{"externalOrderId":"o1","status":"CREATED"}`)(w, r)
	})
	defer closeAPI()

	var raw stampmitra.Response
	st, err := c.Orders.GetStatus(context.Background(), "o1", stampmitra.WithRawResponse(&raw))
	if err != nil {
		panic(err)
	}
	fmt.Println(st.ExternalOrderID, raw.StatusCode, raw.RequestID, raw.Attempts)
	// Output: o1 200 req_77 1
}

func ExampleWithLogger() {
	// Debug logging is off unless you pass a logger; keys and personal data are redacted
	// (this example hides the headers attribute because its content varies by Go version).
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey || a.Key == "elapsed" || a.Key == "url" || a.Key == "headers" {
				return slog.Attr{}
			}
			return a
		},
	})
	c, closeAPI := api(respond(200, `{"referenceId":"SMV-1","valid":true,"registeredName":"Rahul Kumar"}`), stampmitra.WithLogger(slog.New(handler)))
	defer closeAPI()

	_, _ = c.Verify.PAN(context.Background(), &stampmitra.PANVerifyParams{PAN: "ABCDE1234F"}, stampmitra.WithIdempotencyKey("k1"))
	// Output:
	// level=DEBUG msg="stampmitra request" method=POST attempt=1 body="{\"pan\":\"******234F\"}"
	// level=DEBUG msg="stampmitra response" status=200 attempt=1 body="{\"referenceId\":\"SMV-1\",\"registeredName\":\"[REDACTED]\",\"valid\":true}"
}

func ExampleVerifySignature() {
	secret := "whsec_test_0123456789abcdef"
	body := []byte(`{"coreOrderId":"core_order_9f3a1c2b8e4d4a5f","coreEventType":"ORDER_CREATED","occurredAt":"2026-09-25T09:05:00.000Z"}`)
	timestamp := "1760605190"
	signature := stampmitra.SignPayload(body, timestamp, secret) // normally the X-Webhook-Signature header

	now := func() time.Time { return time.Unix(1760605200, 0) } // tests only: omit WithNow in production
	fmt.Println(stampmitra.VerifySignature(body, timestamp, signature, secret, stampmitra.WithNow(now)))

	err := stampmitra.VerifySignature(append(body, ' '), timestamp, signature, secret, stampmitra.WithNow(now))
	var sigErr *stampmitra.SignatureVerificationError
	fmt.Println(errors.As(err, &sigErr), sigErr.Reason)
	// Output:
	// <nil>
	// true signature_mismatch
}

func ExampleConstructEvent() {
	secret := "whsec_test_0123456789abcdef"
	body := []byte(`{"coreOrderId":"core_order_9f3a1c2b8e4d4a5f","coreEventType":"DOCUMENT_READY","occurredAt":"2026-09-25T09:05:00.000Z","documentId":"doc_1"}`)
	timestamp := "1760605190"
	signature := stampmitra.SignPayload(body, timestamp, secret)
	now := func() time.Time { return time.Unix(1760605200, 0) }

	ev, err := stampmitra.ConstructEvent(body, timestamp, signature, secret, stampmitra.WithNow(now))
	if err != nil {
		panic(err)
	}
	switch e := ev.(type) {
	case *stampmitra.OrderDocumentReadyEvent:
		fmt.Println(e.Type(), e.CoreOrderID, string(e.Extra["documentId"]))
	case *stampmitra.UnknownEvent:
		fmt.Println("unknown event", e.RawType)
	}
	// Output: order.document_ready core_order_9f3a1c2b8e4d4a5f "doc_1"
}

func ExampleGenericResponse() {
	c, closeAPI := api(respond(200, `{"states":[{"code":"MH","name":"Maharashtra"},{"code":"DL","name":"Delhi"}],"futureField":true}`))
	defer closeAPI()

	res, err := c.Estamp.ListStates(context.Background())
	if err != nil {
		panic(err)
	}
	var states struct {
		States []struct{ Code, Name string } `json:"states"`
	}
	if err := res.Decode(&states); err != nil {
		panic(err)
	}
	fmt.Println(len(states.States), states.States[0].Name)
	_, ok := res.Get("futureField") // nothing is dropped
	fmt.Println(ok)
	// Output:
	// 2 Maharashtra
	// true
}
