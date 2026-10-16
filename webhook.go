package stampmitra

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Delivery headers sent with every webhook.
const (
	HeaderWebhookSignature = "X-Webhook-Signature"
	HeaderWebhookTimestamp = "X-Webhook-Timestamp"
	HeaderWebhookEventID   = "X-Webhook-Event-Id"
)

// DefaultWebhookTolerance is the default allowed clock difference between the delivery timestamp and now.
const DefaultWebhookTolerance = 300 * time.Second

// Reasons reported in SignatureVerificationError.Reason.
const (
	ReasonInvalidTimestamp = "invalid_timestamp"
	ReasonOutsideTolerance = "timestamp_outside_tolerance"
	ReasonMismatch         = "signature_mismatch"
	ReasonMissingSecret    = "missing_secret"
	ReasonMissingHeaders   = "missing_headers"
	ReasonInvalidPayload   = "invalid_payload"
)

// SignatureVerificationError is returned when a webhook delivery cannot be trusted. It matches
// ErrSignature. It never contains the secret or the signature.
type SignatureVerificationError struct {
	// Reason is one of the Reason* constants.
	Reason string
}

// Error implements error.
func (e *SignatureVerificationError) Error() string {
	return "stampmitra: webhook verification failed: " + e.Reason
}

// Is makes errors.Is(err, ErrSignature) true.
func (e *SignatureVerificationError) Is(target error) bool { return target == ErrSignature }

// WebhookOption tunes webhook verification.
type WebhookOption func(*webhookConfig)

type webhookConfig struct {
	tolerance time.Duration
	now       func() time.Time
}

// WithTolerance sets the maximum allowed difference between the delivery timestamp and the current
// time (default 300s). The check is |now - timestamp| <= tolerance.
func WithTolerance(d time.Duration) WebhookOption { return func(c *webhookConfig) { c.tolerance = d } }

// WithNow overrides the clock, for tests.
func WithNow(now func() time.Time) WebhookOption { return func(c *webhookConfig) { c.now = now } }

func newWebhookConfig(opts []WebhookOption) webhookConfig {
	c := webhookConfig{tolerance: DefaultWebhookTolerance, now: time.Now}
	for _, o := range opts {
		if o != nil {
			o(&c)
		}
	}
	return c
}

// SignPayload computes the delivery signature: lowercase hex HMAC-SHA256(secret, timestamp + "." + payload).
// It is exported for tests and local replays.
func SignPayload(payload []byte, timestamp, secret string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(timestamp))
	m.Write([]byte{'.'})
	m.Write(payload)
	return hex.EncodeToString(m.Sum(nil))
}

// VerifySignature checks a webhook delivery. payload must be the request body EXACTLY as received
// (read it with io.ReadAll before decoding; do not re-encode it), timestamp is the
// X-Webhook-Timestamp header (unix seconds) and signature the X-Webhook-Signature header.
//
// It returns nil when the signature matches (constant-time comparison) and the timestamp is within
// tolerance, otherwise a *SignatureVerificationError. An empty secret is rejected.
func VerifySignature(payload []byte, timestamp, signature, secret string, opts ...WebhookOption) error {
	cfg := newWebhookConfig(opts)
	if secret == "" {
		return &SignatureVerificationError{Reason: ReasonMissingSecret}
	}
	ts, err := parseTimestamp(timestamp)
	if err != nil {
		return &SignatureVerificationError{Reason: ReasonInvalidTimestamp}
	}
	diff := cfg.now().Unix() - ts
	if diff < 0 {
		diff = -diff
	}
	if time.Duration(diff)*time.Second > cfg.tolerance {
		return &SignatureVerificationError{Reason: ReasonOutsideTolerance}
	}
	expected := SignPayload(payload, timestamp, secret)
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return &SignatureVerificationError{Reason: ReasonMismatch}
	}
	return nil
}

func parseTimestamp(s string) (int64, error) {
	if len(s) == 0 || len(s) > 15 {
		return 0, errors.New("bad timestamp")
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, errors.New("bad timestamp")
		}
	}
	return strconv.ParseInt(s, 10, 64)
}

// ConstructEvent verifies the delivery (see VerifySignature) and decodes it into a typed Event.
// Unknown event types decode to *UnknownEvent rather than failing.
func ConstructEvent(payload []byte, timestamp, signature, secret string, opts ...WebhookOption) (Event, error) {
	if err := VerifySignature(payload, timestamp, signature, secret, opts...); err != nil {
		return nil, err
	}
	return ParseEvent(payload)
}

// ConstructEventFromHeaders is ConstructEvent reading X-Webhook-Signature, X-Webhook-Timestamp and
// X-Webhook-Event-Id from h (typically r.Header). The event id header, when present, becomes Base().ID.
func ConstructEventFromHeaders(payload []byte, h http.Header, secret string, opts ...WebhookOption) (Event, error) {
	sig, ts := h.Get(HeaderWebhookSignature), h.Get(HeaderWebhookTimestamp)
	if sig == "" || ts == "" {
		return nil, &SignatureVerificationError{Reason: ReasonMissingHeaders}
	}
	ev, err := ConstructEvent(payload, ts, sig, secret, opts...)
	if err != nil {
		return nil, err
	}
	if id := strings.TrimSpace(h.Get(HeaderWebhookEventID)); id != "" && ev.Base().ID == "" {
		ev.Base().ID = id
	}
	return ev, nil
}

// EventType is the documented name of an order webhook event.
type EventType string

// The eight documented order webhook events.
const (
	EventOrderCreated             EventType = "order.created"
	EventOrderPaymentConfirmed    EventType = "order.payment_confirmed"
	EventOrderFulfilmentStarted   EventType = "order.fulfilment_started"
	EventOrderFulfilmentCompleted EventType = "order.fulfilment_completed"
	EventOrderFailed              EventType = "order.failed"
	EventOrderCancelled           EventType = "order.cancelled"
	EventOrderRefundCompleted     EventType = "order.refund_completed"
	EventOrderDocumentReady       EventType = "order.document_ready"
)

// EventTypes lists the eight documented event types.
var EventTypes = []EventType{
	EventOrderCreated, EventOrderPaymentConfirmed, EventOrderFulfilmentStarted, EventOrderFulfilmentCompleted,
	EventOrderFailed, EventOrderCancelled, EventOrderRefundCompleted, EventOrderDocumentReady,
}

var coreEventTypes = map[string]EventType{
	"ORDER_CREATED":        EventOrderCreated,
	"PAYMENT_CONFIRMED":    EventOrderPaymentConfirmed,
	"FULFILMENT_STARTED":   EventOrderFulfilmentStarted,
	"FULFILMENT_COMPLETED": EventOrderFulfilmentCompleted,
	"ORDER_FAILED":         EventOrderFailed,
	"ORDER_CANCELLED":      EventOrderCancelled,
	"REFUND_COMPLETED":     EventOrderRefundCompleted,
	"DOCUMENT_READY":       EventOrderDocumentReady,
}

// Event is a decoded webhook delivery. Switch on the concrete type:
//
//	switch e := ev.(type) {
//	case *stampmitra.OrderDocumentReadyEvent:
//		...
//	case *stampmitra.UnknownEvent:
//		...
//	}
type Event interface {
	// Type returns the event type ("" for an UnknownEvent without a recognizable type).
	Type() EventType
	// Base returns the common fields.
	Base() *OrderEvent
}

// OrderEvent holds the fields common to all order.* events.
type OrderEvent struct {
	// ID is the delivery's event id (X-Webhook-Event-Id header, or an "id"/"eventId" body field).
	ID string `json:"-"`
	// CoreOrderID identifies the order in the order service.
	CoreOrderID string `json:"coreOrderId"`
	// CoreEventType is the upstream event name, e.g. "ORDER_CREATED".
	CoreEventType string `json:"coreEventType"`
	// OccurredAt is when the event happened (zero if absent or not RFC 3339).
	OccurredAt time.Time `json:"-"`
	// Extra holds every other body field, so additions to the payload are never lost.
	Extra map[string]json.RawMessage `json:"-"`
}

// Base implements Event.
func (e *OrderEvent) Base() *OrderEvent { return e }

// The eight typed events. Each embeds OrderEvent.
type (
	// OrderCreatedEvent is order.created.
	OrderCreatedEvent struct{ OrderEvent }
	// OrderPaymentConfirmedEvent is order.payment_confirmed.
	OrderPaymentConfirmedEvent struct{ OrderEvent }
	// OrderFulfilmentStartedEvent is order.fulfilment_started.
	OrderFulfilmentStartedEvent struct{ OrderEvent }
	// OrderFulfilmentCompletedEvent is order.fulfilment_completed.
	OrderFulfilmentCompletedEvent struct{ OrderEvent }
	// OrderFailedEvent is order.failed.
	OrderFailedEvent struct{ OrderEvent }
	// OrderCancelledEvent is order.cancelled.
	OrderCancelledEvent struct{ OrderEvent }
	// OrderRefundCompletedEvent is order.refund_completed.
	OrderRefundCompletedEvent struct{ OrderEvent }
	// OrderDocumentReadyEvent is order.document_ready.
	OrderDocumentReadyEvent struct{ OrderEvent }
)

// UnknownEvent is any delivery whose type is not one of the eight documented order events.
type UnknownEvent struct {
	OrderEvent
	// RawType is the type string found in the body ("type" or "coreEventType"), if any.
	RawType string
}

// Type implements Event.
func (e *OrderCreatedEvent) Type() EventType { return EventOrderCreated }

// Type implements Event.
func (e *OrderPaymentConfirmedEvent) Type() EventType { return EventOrderPaymentConfirmed }

// Type implements Event.
func (e *OrderFulfilmentStartedEvent) Type() EventType { return EventOrderFulfilmentStarted }

// Type implements Event.
func (e *OrderFulfilmentCompletedEvent) Type() EventType { return EventOrderFulfilmentCompleted }

// Type implements Event.
func (e *OrderFailedEvent) Type() EventType { return EventOrderFailed }

// Type implements Event.
func (e *OrderCancelledEvent) Type() EventType { return EventOrderCancelled }

// Type implements Event.
func (e *OrderRefundCompletedEvent) Type() EventType { return EventOrderRefundCompleted }

// Type implements Event.
func (e *OrderDocumentReadyEvent) Type() EventType { return EventOrderDocumentReady }

// Type implements Event; unknown events have no documented type, so it is always "".
func (e *UnknownEvent) Type() EventType { return "" }

// ParseEvent decodes a webhook body WITHOUT verifying it. Only use it on payloads that were
// verified (or came from a trusted source); prefer ConstructEvent.
//
// The type is derived from coreEventType (ORDER_CREATED, ...); when that is absent or unknown, a
// "type" field holding one of the documented names (order.created, ...) is used.
func ParseEvent(payload []byte) (Event, error) {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(payload, &all); err != nil || all == nil {
		return nil, &SignatureVerificationError{Reason: ReasonInvalidPayload}
	}
	var base OrderEvent
	known := map[string]bool{"coreorderid": true, "coreeventtype": true, "occurredat": true}
	str := func(key string) string {
		var s string
		if raw, ok := all[key]; ok {
			_ = json.Unmarshal(raw, &s)
		}
		return s
	}
	base.CoreOrderID = str("coreOrderId")
	base.CoreEventType = str("coreEventType")
	if s := str("occurredAt"); s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			base.OccurredAt = t
		}
	}
	base.ID = str("eventId")
	if base.ID == "" {
		base.ID = str("id")
	}
	for k, v := range all {
		if known[strings.ToLower(k)] {
			continue
		}
		if base.Extra == nil {
			base.Extra = make(map[string]json.RawMessage)
		}
		base.Extra[k] = append(json.RawMessage(nil), v...)
	}

	typ, ok := coreEventTypes[base.CoreEventType]
	rawType := base.CoreEventType
	if !ok {
		if t := EventType(str("type")); isDocumentedEvent(t) {
			typ, ok = t, true
		}
		if rawType == "" {
			rawType = str("type")
		}
	}
	if !ok {
		return &UnknownEvent{OrderEvent: base, RawType: rawType}, nil
	}
	switch typ {
	case EventOrderCreated:
		return &OrderCreatedEvent{base}, nil
	case EventOrderPaymentConfirmed:
		return &OrderPaymentConfirmedEvent{base}, nil
	case EventOrderFulfilmentStarted:
		return &OrderFulfilmentStartedEvent{base}, nil
	case EventOrderFulfilmentCompleted:
		return &OrderFulfilmentCompletedEvent{base}, nil
	case EventOrderFailed:
		return &OrderFailedEvent{base}, nil
	case EventOrderCancelled:
		return &OrderCancelledEvent{base}, nil
	case EventOrderRefundCompleted:
		return &OrderRefundCompletedEvent{base}, nil
	default:
		return &OrderDocumentReadyEvent{base}, nil
	}
}

func isDocumentedEvent(t EventType) bool {
	for _, d := range EventTypes {
		if d == t {
			return true
		}
	}
	return false
}
