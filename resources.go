package stampmitra

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

func errNilParams(name string) error {
	return &ValidationError{Message: name + ": params must not be nil"}
}

// ---- verify ----------------------------------------------------------------------------------

// VerifyService groups the identity-verification endpoints (POST /v1/verify/*).
//
// Every call is metered and billed, including results that are negative (valid=false, which are
// HTTP 200). Each call carries an Idempotency-Key (auto-generated unless you pass WithIdempotencyKey),
// so automatic retries never bill twice.
type VerifyService struct{ c *Client }

// PAN verifies a PAN (POST /v1/verify/pan).
func (s *VerifyService) PAN(ctx context.Context, p *PANVerifyParams, opts ...RequestOption) (*PANVerification, error) {
	if p == nil {
		return nil, errNilParams("PAN")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[PANVerification](ctx, s.c, http.MethodPost, "/v1/verify/pan", nil, body, true, opts)
}

// GSTIN verifies a GSTIN (POST /v1/verify/gstin).
func (s *VerifyService) GSTIN(ctx context.Context, p *GSTINVerifyParams, opts ...RequestOption) (*GSTINVerification, error) {
	if p == nil {
		return nil, errNilParams("GSTIN")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[GSTINVerification](ctx, s.c, http.MethodPost, "/v1/verify/gstin", nil, body, true, opts)
}

// RC verifies a vehicle registration certificate (POST /v1/verify/rc).
func (s *VerifyService) RC(ctx context.Context, p *RCVerifyParams, opts ...RequestOption) (*RCVerification, error) {
	if p == nil {
		return nil, errNilParams("RC")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[RCVerification](ctx, s.c, http.MethodPost, "/v1/verify/rc", nil, body, true, opts)
}

// VoterID verifies a Voter ID (POST /v1/verify/voter-id).
func (s *VerifyService) VoterID(ctx context.Context, p *VoterIDVerifyParams, opts ...RequestOption) (*VoterIDVerification, error) {
	if p == nil {
		return nil, errNilParams("VoterID")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[VoterIDVerification](ctx, s.c, http.MethodPost, "/v1/verify/voter-id", nil, body, true, opts)
}

// DrivingLicense verifies a driving licence (POST /v1/verify/driving-license). Requires an IP-allowlisted key;
// otherwise the server's 403 message is surfaced verbatim in the returned *APIError.
func (s *VerifyService) DrivingLicense(ctx context.Context, p *DrivingLicenseVerifyParams, opts ...RequestOption) (*DrivingLicenseVerification, error) {
	if p == nil {
		return nil, errNilParams("DrivingLicense")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[DrivingLicenseVerification](ctx, s.c, http.MethodPost, "/v1/verify/driving-license", nil, body, true, opts)
}

// Passport verifies an Indian passport application (POST /v1/verify/passport). Requires an IP-allowlisted key.
func (s *VerifyService) Passport(ctx context.Context, p *PassportVerifyParams, opts ...RequestOption) (*PassportVerification, error) {
	if p == nil {
		return nil, errNilParams("Passport")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[PassportVerification](ctx, s.c, http.MethodPost, "/v1/verify/passport", nil, body, true, opts)
}

// CIN verifies a company CIN (POST /v1/verify/cin).
func (s *VerifyService) CIN(ctx context.Context, p *CINVerifyParams, opts ...RequestOption) (*CINVerification, error) {
	if p == nil {
		return nil, errNilParams("CIN")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[CINVerification](ctx, s.c, http.MethodPost, "/v1/verify/cin", nil, body, true, opts)
}

// Udyam verifies a Udyam registration (POST /v1/verify/udyam).
func (s *VerifyService) Udyam(ctx context.Context, p *UdyamVerifyParams, opts ...RequestOption) (*UdyamVerification, error) {
	if p == nil {
		return nil, errNilParams("Udyam")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[UdyamVerification](ctx, s.c, http.MethodPost, "/v1/verify/udyam", nil, body, true, opts)
}

// PANToGSTIN looks up the GSTINs registered against a PAN (POST /v1/verify/pan-to-gstin).
func (s *VerifyService) PANToGSTIN(ctx context.Context, p *PANToGSTINParams, opts ...RequestOption) (*PANToGSTIN, error) {
	if p == nil {
		return nil, errNilParams("PANToGSTIN")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[PANToGSTIN](ctx, s.c, http.MethodPost, "/v1/verify/pan-to-gstin", nil, body, true, opts)
}

// PANToUdyam looks up the Udyam registration associated with a PAN (POST /v1/verify/pan-to-udyam).
func (s *VerifyService) PANToUdyam(ctx context.Context, p *PANToUdyamParams, opts ...RequestOption) (*PANToUdyam, error) {
	if p == nil {
		return nil, errNilParams("PANToUdyam")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[PANToUdyam](ctx, s.c, http.MethodPost, "/v1/verify/pan-to-udyam", nil, body, true, opts)
}

// BankAccount verifies a bank account by penny drop (POST /v1/verify/bank-account).
func (s *VerifyService) BankAccount(ctx context.Context, p *BankAccountVerifyParams, opts ...RequestOption) (*BankAccountVerification, error) {
	if p == nil {
		return nil, errNilParams("BankAccount")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[BankAccountVerification](ctx, s.c, http.MethodPost, "/v1/verify/bank-account", nil, body, true, opts)
}

// ---- services --------------------------------------------------------------------------------

// ServicesService lists the service catalog (GET /v1/services).
type ServicesService struct{ c *Client }

// List returns the services your organization can see (the API returns a bare JSON array).
func (s *ServicesService) List(ctx context.Context, opts ...RequestOption) ([]Service, error) {
	out, err := call[[]Service](ctx, s.c, http.MethodGet, "/v1/services", nil, nil, false, opts)
	if err != nil {
		return nil, err
	}
	if *out == nil {
		return []Service{}, nil
	}
	return *out, nil
}

// Get returns one service by code (GET /v1/services/{serviceCode}).
func (s *ServicesService) Get(ctx context.Context, serviceCode string, opts ...RequestOption) (*Service, error) {
	seg, err := validatePathID("serviceCode", serviceCode)
	if err != nil {
		return nil, err
	}
	return call[Service](ctx, s.c, http.MethodGet, "/v1/services/"+seg, nil, nil, false, opts)
}

// ---- e-Stamp ---------------------------------------------------------------------------------

// EstampService groups e-Stamp reference data and duty/party helpers. Responses are
// *GenericResponse because the API does not fix their shape; no field is dropped.
type EstampService struct{ c *Client }

// GetSchema returns the input-field schema for an e-Stamp service (GET /v1/estamp/schema/{serviceCode}).
func (s *EstampService) GetSchema(ctx context.Context, serviceCode string, opts ...RequestOption) (*GenericResponse, error) {
	seg, err := validatePathID("serviceCode", serviceCode)
	if err != nil {
		return nil, err
	}
	return call[GenericResponse](ctx, s.c, http.MethodGet, "/v1/estamp/schema/"+seg, nil, nil, false, opts)
}

// ListStates lists supported states (GET /v1/estamp/states).
func (s *EstampService) ListStates(ctx context.Context, opts ...RequestOption) (*GenericResponse, error) {
	return call[GenericResponse](ctx, s.c, http.MethodGet, "/v1/estamp/states", nil, nil, false, opts)
}

// ListInstruments lists instruments for a state (GET /v1/estamp/states/{stateCode}/instruments).
func (s *EstampService) ListInstruments(ctx context.Context, stateCode string, opts ...RequestOption) (*GenericResponse, error) {
	seg, err := validatePathID("stateCode", stateCode)
	if err != nil {
		return nil, err
	}
	return call[GenericResponse](ctx, s.c, http.MethodGet, "/v1/estamp/states/"+seg+"/instruments", nil, nil, false, opts)
}

// ListArticles lists articles for an instrument (GET /v1/estamp/instruments/{instrumentCode}/articles).
func (s *EstampService) ListArticles(ctx context.Context, instrumentCode string, opts ...RequestOption) (*GenericResponse, error) {
	seg, err := validatePathID("instrumentCode", instrumentCode)
	if err != nil {
		return nil, err
	}
	return call[GenericResponse](ctx, s.c, http.MethodGet, "/v1/estamp/instruments/"+seg+"/articles", nil, nil, false, opts)
}

// CalculateDuty calculates stamp duty (POST /v1/estamp/duty/calculate).
func (s *EstampService) CalculateDuty(ctx context.Context, p *EstampDutyParams, opts ...RequestOption) (*GenericResponse, error) {
	if p == nil {
		return nil, errNilParams("CalculateDuty")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[GenericResponse](ctx, s.c, http.MethodPost, "/v1/estamp/duty/calculate", nil, body, true, opts)
}

// AddParties attaches party details to an e-Stamp order (POST /v1/estamp/orders/{orderId}/parties).
// orderID is the externalOrderId you used when creating the order.
func (s *EstampService) AddParties(ctx context.Context, orderID string, p *EstampPartiesParams, opts ...RequestOption) (*GenericResponse, error) {
	seg, err := validatePathID("orderId", orderID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errNilParams("AddParties")
	}
	body := p
	if s.c.validate {
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[GenericResponse](ctx, s.c, http.MethodPost, "/v1/estamp/orders/"+seg+"/parties", nil, body, true, opts)
}

// ---- orders ----------------------------------------------------------------------------------

// OrdersService creates and reads orders. Orders are addressed by YOUR externalOrderId.
type OrdersService struct{ c *Client }

// Create creates an order (POST /v1/orders). The response carries externalOrderId, which is the
// identifier to pass to Get, GetStatus, GetPayment, ListDocuments and GetDocumentDownload.
func (s *OrdersService) Create(ctx context.Context, p *CreateOrderParams, opts ...RequestOption) (*OrderReference, error) {
	if p == nil {
		return nil, errNilParams("Create")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[OrderReference](ctx, s.c, http.MethodPost, "/v1/orders", nil, body, true, opts)
}

// Get returns an order (GET /v1/orders/{id}).
func (s *OrdersService) Get(ctx context.Context, externalOrderID string, opts ...RequestOption) (*OrderDetail, error) {
	seg, err := validatePathID("id", externalOrderID)
	if err != nil {
		return nil, err
	}
	return call[OrderDetail](ctx, s.c, http.MethodGet, "/v1/orders/"+seg, nil, nil, false, opts)
}

// GetStatus returns an order's lifecycle status (GET /v1/orders/{id}/status).
func (s *OrdersService) GetStatus(ctx context.Context, externalOrderID string, opts ...RequestOption) (*OrderStatus, error) {
	seg, err := validatePathID("id", externalOrderID)
	if err != nil {
		return nil, err
	}
	return call[OrderStatus](ctx, s.c, http.MethodGet, "/v1/orders/"+seg+"/status", nil, nil, false, opts)
}

// GetPayment returns the payment associated with an order (GET /v1/orders/{id}/payment).
func (s *OrdersService) GetPayment(ctx context.Context, externalOrderID string, opts ...RequestOption) (*OrderPayment, error) {
	seg, err := validatePathID("id", externalOrderID)
	if err != nil {
		return nil, err
	}
	return call[OrderPayment](ctx, s.c, http.MethodGet, "/v1/orders/"+seg+"/payment", nil, nil, false, opts)
}

// ListDocuments lists documents generated for an order (GET /v1/orders/{id}/documents).
func (s *OrdersService) ListDocuments(ctx context.Context, externalOrderID string, opts ...RequestOption) (*OrderDocuments, error) {
	seg, err := validatePathID("id", externalOrderID)
	if err != nil {
		return nil, err
	}
	return call[OrderDocuments](ctx, s.c, http.MethodGet, "/v1/orders/"+seg+"/documents", nil, nil, false, opts)
}

// GetDocumentDownload returns a short-lived presigned download URL
// (GET /v1/orders/{id}/documents/{documentId}/download).
func (s *OrdersService) GetDocumentDownload(ctx context.Context, externalOrderID, documentID string, opts ...RequestOption) (*DocumentDownloadURL, error) {
	seg, err := validatePathID("id", externalOrderID)
	if err != nil {
		return nil, err
	}
	doc, err := validatePathID("documentId", documentID)
	if err != nil {
		return nil, err
	}
	return call[DocumentDownloadURL](ctx, s.c, http.MethodGet, "/v1/orders/"+seg+"/documents/"+doc+"/download", nil, nil, false, opts)
}

// ---- payments --------------------------------------------------------------------------------

// PaymentsService groups payment resources.
type PaymentsService struct {
	// Intents manages payment intents.
	Intents *PaymentIntentsService
}

// PaymentIntentsService creates and reads payment intents.
type PaymentIntentsService struct{ c *Client }

// Create creates a payment intent (POST /v1/payments/intents). Re-using an idempotency key with the
// same order returns the existing intent.
func (s *PaymentIntentsService) Create(ctx context.Context, p *CreatePaymentIntentParams, opts ...RequestOption) (*PaymentIntent, error) {
	if p == nil {
		return nil, errNilParams("Create")
	}
	body := p
	if s.c.validate {
		var err error
		if body, err = p.normalized(); err != nil {
			return nil, err
		}
	}
	return call[PaymentIntent](ctx, s.c, http.MethodPost, "/v1/payments/intents", nil, body, true, opts)
}

// Get returns a payment intent (GET /v1/payments/intents/{id}).
func (s *PaymentIntentsService) Get(ctx context.Context, id string, opts ...RequestOption) (*PaymentIntent, error) {
	seg, err := validatePathID("id", id)
	if err != nil {
		return nil, err
	}
	return call[PaymentIntent](ctx, s.c, http.MethodGet, "/v1/payments/intents/"+seg, nil, nil, false, opts)
}

// ---- usage -----------------------------------------------------------------------------------

// UsageService reads usage for the organization that owns the key.
type UsageService struct{ c *Client }

// Summary returns the current-period usage summary (GET /v1/usage/summary).
func (s *UsageService) Summary(ctx context.Context, opts ...RequestOption) (*UsageSummary, error) {
	return call[UsageSummary](ctx, s.c, http.MethodGet, "/v1/usage/summary", nil, nil, false, opts)
}

var errCursorStuck = errors.New("nextCursor did not advance")

// ---- logs ------------------------------------------------------------------------------------

// LogsService reads request logs.
type LogsService struct{ c *Client }

// LogsListParams filters GET /v1/logs. Zero values are omitted.
type LogsListParams struct {
	// From is the inclusive lower bound.
	From time.Time
	// To is the exclusive upper bound.
	To time.Time
	// Service filters to one service code, e.g. "pan.verify".
	Service string
	// Status is "success" or "failure".
	Status string
	// Limit is the page size (1-200; the API default is 50).
	Limit int
	// Cursor continues from a previous page's NextCursor.
	Cursor string
}

func (p *LogsListParams) query(validate bool) (url.Values, error) {
	q := url.Values{}
	if p == nil {
		return q, nil
	}
	if !p.From.IsZero() {
		q.Set("from", p.From.UTC().Format(time.RFC3339Nano))
	}
	if !p.To.IsZero() {
		q.Set("to", p.To.UTC().Format(time.RFC3339Nano))
	}
	if p.Service != "" {
		q.Set("service", p.Service)
	}
	if p.Status != "" {
		if validate && p.Status != "success" && p.Status != "failure" {
			return nil, &ValidationError{Field: "status", Message: `must be "success" or "failure"`}
		}
		q.Set("status", p.Status)
	}
	if p.Limit != 0 {
		if validate && (p.Limit < 1 || p.Limit > 200) {
			return nil, &ValidationError{Field: "limit", Message: "must be between 1 and 200"}
		}
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	return q, nil
}

// List returns one page of request logs (GET /v1/logs). p may be nil.
func (s *LogsService) List(ctx context.Context, p *LogsListParams, opts ...RequestOption) (*LogsPage, error) {
	q, err := p.query(s.c.validate)
	if err != nil {
		return nil, err
	}
	return call[LogsPage](ctx, s.c, http.MethodGet, "/v1/logs", q, nil, false, opts)
}

// ListAll returns an iterator that follows nextCursor across pages, fetching lazily:
//
//	it := client.Logs.ListAll(&stampmitra.LogsListParams{Limit: 100})
//	for it.Next(ctx) {
//		entry := it.Log()
//	}
//	if err := it.Err(); err != nil { ... }
//
// ListAll performs no I/O itself; each page is fetched by Next using the context passed to it.
// p.Cursor, when set, is the starting cursor. p is copied.
func (s *LogsService) ListAll(p *LogsListParams, opts ...RequestOption) *LogsIterator {
	it := &LogsIterator{c: s.c, opts: opts}
	if p != nil {
		it.params = *p
	}
	it.cursor = it.params.Cursor
	return it
}

// LogsIterator iterates over all log entries across pages. It is not safe for concurrent use.
type LogsIterator struct {
	c      *Client
	params LogsListParams
	opts   []RequestOption

	items  []LogEntry
	idx    int
	cursor string
	done   bool
	cur    *LogEntry
	err    error
}

// Next advances to the next log entry, fetching the next page when needed. It returns false when the
// entries are exhausted or an error occurred (check Err).
func (it *LogsIterator) Next(ctx context.Context) bool {
	if it.err != nil {
		return false
	}
	for {
		if it.idx < len(it.items) {
			it.cur = &it.items[it.idx]
			it.idx++
			return true
		}
		it.cur = nil
		if it.done {
			return false
		}
		p := it.params
		p.Cursor = it.cursor
		page, err := it.c.Logs.List(ctx, &p, it.opts...)
		if err != nil {
			it.err = err
			return false
		}
		it.items, it.idx = page.Items, 0
		next := ""
		if page.NextCursor != nil {
			next = *page.NextCursor
		}
		switch next {
		case "":
			it.done = true
		case it.cursor:
			it.err = &DecodeError{Status: http.StatusOK, Err: errCursorStuck}
			it.done = true
			if len(it.items) == 0 {
				return false
			}
		}
		it.cursor = next
	}
}

// Log returns the current entry; it is nil before the first successful Next and after exhaustion.
func (it *LogsIterator) Log() *LogEntry { return it.cur }

// Err returns the error that stopped iteration, if any.
func (it *LogsIterator) Err() error { return it.err }

// ---- webhook events --------------------------------------------------------------------------

// WebhooksService groups webhook-related API resources. For verifying incoming deliveries see
// VerifySignature and ConstructEvent.
type WebhooksService struct {
	// Events lists and replays webhook deliveries.
	Events *WebhookEventsService
}

// WebhookEventsService lists and replays your organization's webhook deliveries.
type WebhookEventsService struct{ c *Client }

// WebhookEventsListParams filters GET /v1/webhooks/events. Zero values are omitted.
type WebhookEventsListParams struct {
	Limit  int
	Cursor string
}

// List returns a page of webhook deliveries (GET /v1/webhooks/events). p may be nil. The shape is
// not fixed by the API, so the result is a *GenericResponse (see Items and NextCursor).
func (s *WebhookEventsService) List(ctx context.Context, p *WebhookEventsListParams, opts ...RequestOption) (*GenericResponse, error) {
	q := url.Values{}
	if p != nil {
		if p.Limit != 0 {
			if s.c.validate && p.Limit < 1 {
				return nil, &ValidationError{Field: "limit", Message: "must be at least 1"}
			}
			q.Set("limit", strconv.Itoa(p.Limit))
		}
		if p.Cursor != "" {
			q.Set("cursor", p.Cursor)
		}
	}
	return call[GenericResponse](ctx, s.c, http.MethodGet, "/v1/webhooks/events", q, nil, false, opts)
}

// Replay re-delivers one webhook event (POST /v1/webhooks/events/{id}/replay). Replay carries no
// automatic Idempotency-Key and is therefore never auto-retried unless you pass WithIdempotencyKey.
func (s *WebhookEventsService) Replay(ctx context.Context, eventID string, opts ...RequestOption) (*GenericResponse, error) {
	seg, err := validatePathID("id", eventID)
	if err != nil {
		return nil, err
	}
	return call[GenericResponse](ctx, s.c, http.MethodPost, "/v1/webhooks/events/"+seg+"/replay", nil, nil, false, opts)
}
