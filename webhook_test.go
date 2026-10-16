package stampmitra

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type sigVectors struct {
	Now     int64 `json:"now"`
	Vectors []struct {
		Name      string `json:"name"`
		Secret    string `json:"secret"`
		Timestamp string `json:"timestamp"`
		Payload   string `json:"payload"`
		Signature string `json:"signature"`
		Expected  struct {
			Valid  bool   `json:"valid"`
			Reason string `json:"reason"`
		} `json:"expected"`
	} `json:"vectors"`
}

func TestWebhookSignatureVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/webhook-signatures.json")
	if err != nil {
		t.Fatal(err)
	}
	var f sigVectors
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Vectors) < 20 {
		t.Fatalf("only %d vectors", len(f.Vectors))
	}
	now := func() time.Time { return time.Unix(f.Now, 0) }
	for _, v := range f.Vectors {
		t.Run(v.Name, func(t *testing.T) {
			err := VerifySignature([]byte(v.Payload), v.Timestamp, v.Signature, v.Secret, WithNow(now))
			if v.Expected.Valid {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				if _, err := ConstructEvent([]byte(v.Payload), v.Timestamp, v.Signature, v.Secret, WithNow(now)); err != nil {
					t.Fatalf("ConstructEvent: %v", err)
				}
				return
			}
			var se *SignatureVerificationError
			if !errors.As(err, &se) || !errors.Is(err, ErrSignature) {
				t.Fatalf("want *SignatureVerificationError, got %v", err)
			}
			if se.Reason != v.Expected.Reason {
				t.Fatalf("reason = %q, want %q", se.Reason, v.Expected.Reason)
			}
			if strings.Contains(err.Error(), v.Secret) || (v.Signature != "" && strings.Contains(err.Error(), v.Signature)) {
				t.Fatal("error leaks secret or signature")
			}
			if _, err := ConstructEvent([]byte(v.Payload), v.Timestamp, v.Signature, v.Secret, WithNow(now)); err == nil {
				t.Fatal("ConstructEvent must fail on an invalid delivery")
			}
		})
	}
}

func TestWebhookEmptySecretRejected(t *testing.T) {
	sig := SignPayload([]byte("{}"), "1760605190", "")
	err := VerifySignature([]byte("{}"), "1760605190", sig, "", WithNow(func() time.Time { return time.Unix(1760605200, 0) }))
	var se *SignatureVerificationError
	if !errors.As(err, &se) || se.Reason != ReasonMissingSecret {
		t.Fatalf("got %v", err)
	}
}

func TestWebhookToleranceOption(t *testing.T) {
	now := func() time.Time { return time.Unix(1760605200, 0) }
	body := []byte(`{"coreOrderId":"x","coreEventType":"ORDER_CREATED","occurredAt":"2026-09-25T09:05:00.000Z"}`)
	ts := "1760605100" // 100s old
	sig := SignPayload(body, ts, "s3cret")
	if err := VerifySignature(body, ts, sig, "s3cret", WithNow(now)); err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(body, ts, sig, "s3cret", WithNow(now), WithTolerance(50*time.Second)); err == nil {
		t.Fatal("must be outside a 50s tolerance")
	}
	if err := VerifySignature(body, ts, sig, "s3cret", WithNow(now), WithTolerance(100*time.Second)); err != nil {
		t.Fatal("exactly at tolerance must pass:", err)
	}
	// the default uses the real clock: an old timestamp fails
	if err := VerifySignature(body, ts, sig, "s3cret"); err == nil {
		t.Fatal("2025 timestamp must be stale against the real clock")
	}
}

func TestWebhookRealClockFreshDelivery(t *testing.T) {
	body := []byte(`{"coreOrderId":"x","coreEventType":"ORDER_FAILED","occurredAt":"2026-09-25T09:05:00.000Z"}`)
	ts := time.Now().Unix()
	tsStr := strconv.FormatInt(ts, 10)
	sig := SignPayload(body, tsStr, "whsec_x")
	ev, err := ConstructEvent(body, tsStr, sig, "whsec_x")
	if err != nil {
		t.Fatal(err)
	}
	if ev.Type() != EventOrderFailed {
		t.Fatalf("type = %v", ev.Type())
	}
}

type fixtureFile struct {
	Event    string            `json:"event"`
	Secret   string            `json:"secret"`
	Now      int64             `json:"now"`
	Headers  map[string]string `json:"headers"`
	RawBody  string            `json:"rawBody"`
	Expected struct{ Valid bool }
}

func TestConstructEventForAllEightEvents(t *testing.T) {
	files, err := filepath.Glob("testdata/webhooks/*.json")
	if err != nil || len(files) != 8 {
		t.Fatalf("expected 8 webhook fixtures, got %d (%v)", len(files), err)
	}
	want := map[string]string{
		"order.created": "*stampmitra.OrderCreatedEvent", "order.payment_confirmed": "*stampmitra.OrderPaymentConfirmedEvent",
		"order.fulfilment_started": "*stampmitra.OrderFulfilmentStartedEvent", "order.fulfilment_completed": "*stampmitra.OrderFulfilmentCompletedEvent",
		"order.failed": "*stampmitra.OrderFailedEvent", "order.cancelled": "*stampmitra.OrderCancelledEvent",
		"order.refund_completed": "*stampmitra.OrderRefundCompletedEvent", "order.document_ready": "*stampmitra.OrderDocumentReadyEvent",
	}
	seen := map[string]bool{}
	for _, f := range files {
		raw, _ := os.ReadFile(f)
		var fx fixtureFile
		if err := json.Unmarshal(raw, &fx); err != nil {
			t.Fatal(err)
		}
		h := http.Header{}
		for k, v := range fx.Headers {
			h.Set(k, v)
		}
		ev, err := ConstructEventFromHeaders([]byte(fx.RawBody), h, fx.Secret, WithNow(func() time.Time { return time.Unix(fx.Now, 0) }))
		if err != nil {
			t.Fatalf("%s: %v", fx.Event, err)
		}
		if string(ev.Type()) != fx.Event {
			t.Errorf("%s: Type() = %q", f, ev.Type())
		}
		if got := typeName(ev); got != want[fx.Event] {
			t.Errorf("%s: concrete type %s, want %s", fx.Event, got, want[fx.Event])
		}
		b := ev.Base()
		if b.CoreOrderID != "core_order_9f3a1c2b8e4d4a5f" || b.CoreEventType == "" || b.OccurredAt.IsZero() || b.ID != fx.Headers["X-Webhook-Event-Id"] {
			t.Errorf("%s: base = %+v", fx.Event, b)
		}
		seen[fx.Event] = true
	}
	if len(seen) != 8 {
		t.Fatalf("seen %d distinct events", len(seen))
	}
}

func typeName(v any) string {
	switch v.(type) {
	case *OrderCreatedEvent:
		return "*stampmitra.OrderCreatedEvent"
	case *OrderPaymentConfirmedEvent:
		return "*stampmitra.OrderPaymentConfirmedEvent"
	case *OrderFulfilmentStartedEvent:
		return "*stampmitra.OrderFulfilmentStartedEvent"
	case *OrderFulfilmentCompletedEvent:
		return "*stampmitra.OrderFulfilmentCompletedEvent"
	case *OrderFailedEvent:
		return "*stampmitra.OrderFailedEvent"
	case *OrderCancelledEvent:
		return "*stampmitra.OrderCancelledEvent"
	case *OrderRefundCompletedEvent:
		return "*stampmitra.OrderRefundCompletedEvent"
	case *OrderDocumentReadyEvent:
		return "*stampmitra.OrderDocumentReadyEvent"
	case *UnknownEvent:
		return "*stampmitra.UnknownEvent"
	}
	return "?"
}

func TestParseEventExtrasAndFallbacks(t *testing.T) {
	ev, err := ParseEvent([]byte(`{"coreOrderId":"c1","coreEventType":"DOCUMENT_READY","occurredAt":"2026-09-25T09:05:00.000Z","documentId":"d1","nested":{"a":1}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ev.(*OrderDocumentReadyEvent); !ok {
		t.Fatalf("got %T", ev)
	}
	if string(ev.Base().Extra["documentId"]) != `"d1"` || string(ev.Base().Extra["nested"]) != `{"a":1}` || len(ev.Base().Extra) != 2 {
		t.Fatalf("extra = %v", ev.Base().Extra)
	}

	// documented `type` is used when coreEventType is missing
	ev, _ = ParseEvent([]byte(`{"type":"order.cancelled","id":"evt_9"}`))
	if _, ok := ev.(*OrderCancelledEvent); !ok || ev.Base().ID != "evt_9" {
		t.Fatalf("got %T %+v", ev, ev.Base())
	}

	// unknown types never fail
	ev, err = ParseEvent([]byte(`{"type":"verification.completed","coreOrderId":"c"}`))
	u, ok := ev.(*UnknownEvent)
	if err != nil || !ok || u.RawType != "verification.completed" || u.Type() != "" {
		t.Fatalf("got %T %v", ev, err)
	}
	ev, _ = ParseEvent([]byte(`{"coreEventType":"SOMETHING_NEW"}`))
	if u, ok := ev.(*UnknownEvent); !ok || u.RawType != "SOMETHING_NEW" {
		t.Fatalf("got %T", ev)
	}
	// bad occurredAt does not fail
	ev, err = ParseEvent([]byte(`{"coreEventType":"ORDER_CREATED","occurredAt":"yesterday"}`))
	if err != nil || !ev.Base().OccurredAt.IsZero() {
		t.Fatalf("%v %+v", err, ev)
	}
}

func TestParseEventRejectsNonObjects(t *testing.T) {
	for _, in := range []string{"", "null", "[]", `"x"`, "{", "42"} {
		_, err := ParseEvent([]byte(in))
		var se *SignatureVerificationError
		if !errors.As(err, &se) || se.Reason != ReasonInvalidPayload {
			t.Errorf("%q: got %v", in, err)
		}
	}
}

func TestConstructEventFromHeadersMissingHeaders(t *testing.T) {
	_, err := ConstructEventFromHeaders([]byte("{}"), http.Header{}, "s")
	var se *SignatureVerificationError
	if !errors.As(err, &se) || se.Reason != ReasonMissingHeaders {
		t.Fatalf("got %v", err)
	}
}

func TestSignatureMustBeOverRawBytes(t *testing.T) {
	now := func() time.Time { return time.Unix(1760605200, 0) }
	body := `{"coreEventType":"ORDER_CREATED","coreOrderId":"x"}`
	ts := "1760605190"
	sig := SignPayload([]byte(body), ts, "k")
	pretty := `{ "coreEventType": "ORDER_CREATED", "coreOrderId": "x" }`
	if err := VerifySignature([]byte(pretty), ts, sig, "k", WithNow(now)); err == nil {
		t.Fatal("re-serialised body must not verify")
	}
	if err := VerifySignature([]byte(body), ts, strings.ToUpper(sig), "k", WithNow(now)); err == nil {
		t.Fatal("upper-case hex must not verify")
	}
}
