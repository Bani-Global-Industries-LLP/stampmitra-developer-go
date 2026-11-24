package stampmitra

import (
	"bytes"
	"context"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

// Limits documented by the API.
const (
	maxEstampStamps    = 5000
	maxStampQuantity   = 1000
	maxDocumentBytes   = 10 << 20
	maxOrdersPageLimit = 100
)

// ---- iterator ---------------------------------------------------------------------------------

// Iterator walks a cursor-paginated list lazily, following nextCursor. It is not safe for concurrent use.
//
//	it := client.Estamp.Orders.ListAll(nil)
//	for it.Next(ctx) {
//		fmt.Println(it.Item().ID)
//	}
//	if err := it.Err(); err != nil { ... }
type Iterator[T any] struct {
	fetch  func(ctx context.Context, cursor string) ([]T, string, error)
	items  []T
	idx    int
	cursor string
	done   bool
	cur    *T
	err    error
}

// Next advances to the next item, fetching the next page when needed. It returns false when the
// list is exhausted or an error occurred (see Err).
func (it *Iterator[T]) Next(ctx context.Context) bool {
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
		items, next, err := it.fetch(ctx, it.cursor)
		if err != nil {
			it.err = err
			return false
		}
		it.items, it.idx = items, 0
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

// Item returns the current item; nil before the first Next and after exhaustion.
func (it *Iterator[T]) Item() *T { return it.cur }

// Err returns the error that stopped iteration, if any.
func (it *Iterator[T]) Err() error { return it.err }

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ---- e-Stamp reference data -------------------------------------------------------------------

// StatesAvailabilityParams filters GET /v1/estamp/states/availability.
type StatesAvailabilityParams struct {
	// ActiveOnly returns only the states that accept new orders right now.
	ActiveOnly bool
}

// GetStatesAvailability reports which states accept new stamp orders right now
// (GET /v1/estamp/states/availability). p may be nil.
func (s *EstampService) GetStatesAvailability(ctx context.Context, p *StatesAvailabilityParams, opts ...RequestOption) (*EstampStatesAvailability, error) {
	q := url.Values{}
	if p != nil && p.ActiveOnly {
		q.Set("active", "true")
	}
	return call[EstampStatesAvailability](ctx, s.c, http.MethodGet, "/v1/estamp/states/availability", q, nil, false, opts)
}

// GetState returns one state's detail (GET /v1/estamp/states/{stateCode}).
func (s *EstampService) GetState(ctx context.Context, stateCode string, opts ...RequestOption) (*EstampState, error) {
	seg, err := validatePathID("stateCode", stateCode)
	if err != nil {
		return nil, err
	}
	return call[EstampState](ctx, s.c, http.MethodGet, "/v1/estamp/states/"+seg, nil, nil, false, opts)
}

// StateRequirementsParams narrows GET /v1/estamp/states/{stateCode}/requirements. Zero values are omitted.
type StateRequirementsParams struct {
	// ArticleCode is an article code from ListInstruments (at most 60 characters).
	ArticleCode string
	// Denomination is a stamp duty amount in rupees; must be greater than 0 when set.
	Denomination float64
	// Category is CategoryEStamp or CategoryTraditional.
	Category string
}

// GetStateRequirements returns the field requirements for a state and optionally an article,
// denomination and category, with a generated ExampleBody (GET /v1/estamp/states/{stateCode}/requirements).
// p may be nil.
func (s *EstampService) GetStateRequirements(ctx context.Context, stateCode string, p *StateRequirementsParams, opts ...RequestOption) (*EstampRequirements, error) {
	seg, err := validatePathID("stateCode", stateCode)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if p != nil {
		if p.ArticleCode != "" {
			if s.c.validate && len(p.ArticleCode) > 60 {
				return nil, &ValidationError{Field: "articleCode", Message: "must be at most 60 characters"}
			}
			q.Set("articleCode", p.ArticleCode)
		}
		if p.Denomination != 0 {
			if s.c.validate && (p.Denomination < 0 || math.IsNaN(p.Denomination) || math.IsInf(p.Denomination, 0)) {
				return nil, &ValidationError{Field: "denomination", Message: "must be greater than 0"}
			}
			q.Set("denomination", strconv.FormatFloat(p.Denomination, 'f', -1, 64))
		}
		if p.Category != "" {
			if s.c.validate && p.Category != CategoryEStamp && p.Category != CategoryTraditional {
				return nil, &ValidationError{Field: "category", Message: `must be "e_stamp" or "traditional_stamp"`}
			}
			q.Set("category", p.Category)
		}
	}
	return call[EstampRequirements](ctx, s.c, http.MethodGet, "/v1/estamp/states/"+seg+"/requirements", q, nil, false, opts)
}

// ListDistricts lists a state's districts (GET /v1/estamp/states/{stateCode}/districts).
func (s *EstampService) ListDistricts(ctx context.Context, stateCode string, opts ...RequestOption) ([]EstampDistrict, error) {
	seg, err := validatePathID("stateCode", stateCode)
	if err != nil {
		return nil, err
	}
	out, err := call[[]EstampDistrict](ctx, s.c, http.MethodGet, "/v1/estamp/states/"+seg+"/districts", nil, nil, false, opts)
	if err != nil {
		return nil, err
	}
	if *out == nil {
		return []EstampDistrict{}, nil
	}
	return *out, nil
}

// ListSROs lists a district's sub-registrar offices (GET /v1/estamp/states/{stateCode}/districts/{districtCode}/sros).
// The list is empty when the district has none on file.
func (s *EstampService) ListSROs(ctx context.Context, stateCode, districtCode string, opts ...RequestOption) ([]EstampSRO, error) {
	seg, err := validatePathID("stateCode", stateCode)
	if err != nil {
		return nil, err
	}
	dseg, err := validatePathID("districtCode", districtCode)
	if err != nil {
		return nil, err
	}
	out, err := call[[]EstampSRO](ctx, s.c, http.MethodGet, "/v1/estamp/states/"+seg+"/districts/"+dseg+"/sros", nil, nil, false, opts)
	if err != nil {
		return nil, err
	}
	if *out == nil {
		return []EstampSRO{}, nil
	}
	return *out, nil
}

// GetWallet reads both e-Stamp wallets at once (GET /v1/estamp/wallet). The Stamp Duty Wallet is at
// the top level and the Service Fee Wallet under ServiceFee. Needs both wallet read scopes.
func (s *EstampService) GetWallet(ctx context.Context, opts ...RequestOption) (*EstampWallet, error) {
	return call[EstampWallet](ctx, s.c, http.MethodGet, "/v1/estamp/wallet", nil, nil, false, opts)
}

// ---- e-Stamp orders ---------------------------------------------------------------------------

// EstampOrdersService validates, creates and reads e-Stamp orders (/v1/estamp/orders). One order
// covers one state, up to 5000 stamp lines.
type EstampOrdersService struct{ c *Client }

func (p *EstampOrderParams) check() error {
	if len(p.Stamps) == 0 {
		return &ValidationError{Field: "stamps", Message: "at least one stamp line is required"}
	}
	if len(p.Stamps) > maxEstampStamps {
		return &ValidationError{Field: "stamps", Message: "at most " + strconv.Itoa(maxEstampStamps) + " stamp lines per order"}
	}
	state := ""
	for i, l := range p.Stamps {
		f := func(n string) string { return "stamps[" + strconv.Itoa(i) + "]." + n }
		if strings.TrimSpace(l.State) == "" {
			return &ValidationError{Field: f("state"), Message: "is required"}
		}
		if state == "" {
			state = strings.ToUpper(strings.TrimSpace(l.State))
		} else if strings.ToUpper(strings.TrimSpace(l.State)) != state {
			return &ValidationError{Field: f("state"), Message: "one order covers exactly one state"}
		}
		if strings.TrimSpace(l.ArticleCode) == "" {
			return &ValidationError{Field: f("articleCode"), Message: "is required"}
		}
		if !(l.Denomination > 0) || math.IsInf(l.Denomination, 0) {
			return &ValidationError{Field: f("denomination"), Message: "must be greater than 0"}
		}
		if l.FirstParty.IsZero() {
			return &ValidationError{Field: f("firstParty"), Message: "is required"}
		}
		if l.Quantity < 0 || l.Quantity > maxStampQuantity {
			return &ValidationError{Field: f("quantity"), Message: "must be between 1 and " + strconv.Itoa(maxStampQuantity)}
		}
		if l.PANCardImage != "" && !strings.HasPrefix(l.PANCardImage, "https://") {
			return &ValidationError{Field: f("panCardImage"), Message: "must be an https link"}
		}
	}
	return nil
}

// Validate runs the order validation without saving or charging anything
// (POST /v1/estamp/orders/validate). A rejected body is a 400 *APIError whose FieldErrors list each
// finding; a body with missing details is a successful call with Valid false. Validate sends no
// Idempotency-Key and, being a dry run, is retried like a GET.
func (s *EstampOrdersService) Validate(ctx context.Context, p *EstampOrderParams, opts ...RequestOption) (*EstampValidateResult, error) {
	if p == nil {
		return nil, errNilParams("Validate")
	}
	if s.c.validate {
		if err := p.check(); err != nil {
			return nil, err
		}
	}
	return callWith[EstampValidateResult](ctx, s.c, http.MethodPost, "/v1/estamp/orders/validate", nil, p, false, retrySafe, opts)
}

// Create creates an e-Stamp order (POST /v1/estamp/orders). This debits real wallets. The API
// requires an Idempotency-Key: when you do not pass one with WithIdempotencyKey the SDK generates
// a UUIDv4 and re-uses it on every automatic retry, so a retry never creates a second order. If
// your program itself may run twice, derive the key from your own order reference.
func (s *EstampOrdersService) Create(ctx context.Context, p *EstampOrderParams, opts ...RequestOption) (*EstampOrder, error) {
	if p == nil {
		return nil, errNilParams("Create")
	}
	if s.c.validate {
		if err := p.check(); err != nil {
			return nil, err
		}
	}
	return call[EstampOrder](ctx, s.c, http.MethodPost, "/v1/estamp/orders", nil, p, true, opts)
}

// EstampOrdersListParams filters GET /v1/estamp/orders. Zero values are omitted.
type EstampOrdersListParams struct {
	// Page is a page number (pages beyond 20 are refused by the API; continue with Cursor).
	Page int
	// Limit is the page size (1-100; the API default is 20).
	Limit int
	// Cursor continues from a previous page's NextCursor.
	Cursor string
}

func (p *EstampOrdersListParams) query(validate bool) (url.Values, error) {
	q := url.Values{}
	if p == nil {
		return q, nil
	}
	if p.Page != 0 {
		if validate && p.Page < 1 {
			return nil, &ValidationError{Field: "page", Message: "must be at least 1"}
		}
		q.Set("page", strconv.Itoa(p.Page))
	}
	if p.Limit != 0 {
		if validate && (p.Limit < 1 || p.Limit > maxOrdersPageLimit) {
			return nil, &ValidationError{Field: "limit", Message: "must be between 1 and 100"}
		}
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	return q, nil
}

// List returns one page of your organization's orders, newest first (GET /v1/estamp/orders). p may be nil.
func (s *EstampOrdersService) List(ctx context.Context, p *EstampOrdersListParams, opts ...RequestOption) (*EstampOrderList, error) {
	q, err := p.query(s.c.validate)
	if err != nil {
		return nil, err
	}
	return call[EstampOrderList](ctx, s.c, http.MethodGet, "/v1/estamp/orders", q, nil, false, opts)
}

// ListAll returns an iterator over all orders that follows nextCursor. p may be nil; p.Cursor is the
// starting cursor and p.Page is ignored.
func (s *EstampOrdersService) ListAll(p *EstampOrdersListParams, opts ...RequestOption) *Iterator[EstampOrder] {
	var base EstampOrdersListParams
	if p != nil {
		base = *p
	}
	base.Page = 0
	return &Iterator[EstampOrder]{cursor: base.Cursor, fetch: func(ctx context.Context, cursor string) ([]EstampOrder, string, error) {
		q := base
		q.Cursor = cursor
		page, err := s.List(ctx, &q, opts...)
		if err != nil {
			return nil, "", err
		}
		return page.Items, derefString(page.NextCursor), nil
	}}
}

// Get returns an order by its id (GET /v1/estamp/orders/{id}).
func (s *EstampOrdersService) Get(ctx context.Context, id string, opts ...RequestOption) (*EstampOrder, error) {
	seg, err := validatePathID("id", id)
	if err != nil {
		return nil, err
	}
	return call[EstampOrder](ctx, s.c, http.MethodGet, "/v1/estamp/orders/"+seg, nil, nil, false, opts)
}

// Cancel cancels an order (POST /v1/estamp/orders/{id}/cancel). It is sent exactly once and never
// retried automatically; after an error, read the order with Get before trying again.
func (s *EstampOrdersService) Cancel(ctx context.Context, id string, opts ...RequestOption) (*EstampOrder, error) {
	seg, err := validatePathID("id", id)
	if err != nil {
		return nil, err
	}
	return callWith[EstampOrder](ctx, s.c, http.MethodPost, "/v1/estamp/orders/"+seg+"/cancel", nil, nil, false, retryNever, opts)
}

// Export returns a small CSV export of an order for reconciliation (GET /v1/estamp/orders/{id}/export).
func (s *EstampOrdersService) Export(ctx context.Context, id string, opts ...RequestOption) (*EstampOrderExport, error) {
	seg, err := validatePathID("id", id)
	if err != nil {
		return nil, err
	}
	return call[EstampOrderExport](ctx, s.c, http.MethodGet, "/v1/estamp/orders/"+seg+"/export", nil, nil, false, opts)
}

// ListStamps returns every stamp of an order (GET /v1/estamp/orders/{id}/stamps).
func (s *EstampOrdersService) ListStamps(ctx context.Context, id string, opts ...RequestOption) (*EstampStampList, error) {
	seg, err := validatePathID("id", id)
	if err != nil {
		return nil, err
	}
	return call[EstampStampList](ctx, s.c, http.MethodGet, "/v1/estamp/orders/"+seg+"/stamps", nil, nil, false, opts)
}

// EstampDocumentUploadParams is the multipart body of UploadDocument.
type EstampDocumentUploadParams struct {
	// File is the payer's PAN card: PDF, JPEG or PNG, at most 10 MB. It is read fully into memory.
	File io.Reader
	// FileName is sent as the file part's filename. Optional; the API never logs it.
	FileName string
	// ContentType is the declared type (application/pdf, image/jpeg or image/png). When empty it is
	// taken from the FileName extension, else detected from the first bytes. The API checks the first
	// bytes against the declared type.
	ContentType string
	// Type is the optional type part; the only accepted value is DocumentTypePANCard.
	Type string
}

// UploadDocument attaches the payer's PAN card to an order as multipart/form-data
// (POST /v1/estamp/orders/{orderId}/documents). It is optional and advisory: an order is never held
// back for want of it. Like Create it carries an auto-generated Idempotency-Key that is re-used on
// retries; the whole body is re-sent unchanged.
func (s *EstampOrdersService) UploadDocument(ctx context.Context, orderID string, p *EstampDocumentUploadParams, opts ...RequestOption) (*EstampDocument, error) {
	seg, err := validatePathID("orderId", orderID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.File == nil {
		return nil, &ValidationError{Field: "file", Message: "is required"}
	}
	data, err := io.ReadAll(io.LimitReader(p.File, maxDocumentBytes+1))
	if err != nil {
		return nil, &ValidationError{Field: "file", Message: "cannot be read"}
	}
	if len(data) == 0 {
		return nil, &ValidationError{Field: "file", Message: "is empty"}
	}
	if s.c.validate && len(data) > maxDocumentBytes {
		return nil, &ValidationError{Field: "file", Message: "is larger than 10 MB"}
	}
	ct := p.ContentType
	if ct == "" {
		switch strings.ToLower(filepath.Ext(p.FileName)) {
		case ".pdf":
			ct = "application/pdf"
		case ".jpg", ".jpeg":
			ct = "image/jpeg"
		case ".png":
			ct = "image/png"
		default:
			ct = http.DetectContentType(data)
		}
	}
	if s.c.validate {
		switch ct {
		case "application/pdf", "image/jpeg", "image/png":
		default:
			return nil, &ValidationError{Field: "contentType", Message: "must be application/pdf, image/jpeg or image/png"}
		}
		if p.Type != "" && p.Type != DocumentTypePANCard {
			return nil, &ValidationError{Field: "type", Message: `must be "pan_card"`}
		}
	}
	name := p.FileName
	if name == "" {
		name = "pan-card"
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+quoteFileName(filepath.Base(name))+`"`)
	h.Set("Content-Type", ct)
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, &ValidationError{Field: "file", Message: "cannot be encoded"}
	}
	if _, err := part.Write(data); err != nil {
		return nil, &ValidationError{Field: "file", Message: "cannot be encoded"}
	}
	if p.Type != "" {
		if err := w.WriteField("type", p.Type); err != nil {
			return nil, &ValidationError{Field: "type", Message: "cannot be encoded"}
		}
	}
	if err := w.Close(); err != nil {
		return nil, &ValidationError{Message: "multipart body cannot be encoded"}
	}
	body := rawBody{contentType: w.FormDataContentType(), data: buf.Bytes()}
	return call[EstampDocument](ctx, s.c, http.MethodPost, "/v1/estamp/orders/"+seg+"/documents", nil, body, true, opts)
}

// ---- e-Stamp stamps ---------------------------------------------------------------------------

// EstampStampsService reads and changes single stamps (/v1/estamp/stamps).
type EstampStampsService struct{ c *Client }

// Get returns one stamp, including the state of its payer PAN card (GET /v1/estamp/stamps/{stampId}).
func (s *EstampStampsService) Get(ctx context.Context, stampID string, opts ...RequestOption) (*EstampStamp, error) {
	seg, err := validatePathID("stampId", stampID)
	if err != nil {
		return nil, err
	}
	return call[EstampStamp](ctx, s.c, http.MethodGet, "/v1/estamp/stamps/"+seg, nil, nil, false, opts)
}

// Cancel cancels a stamp (POST /v1/estamp/stamps/{stampId}/cancel). Never retried automatically.
func (s *EstampStampsService) Cancel(ctx context.Context, stampID string, opts ...RequestOption) (*EstampStamp, error) {
	seg, err := validatePathID("stampId", stampID)
	if err != nil {
		return nil, err
	}
	return callWith[EstampStamp](ctx, s.c, http.MethodPost, "/v1/estamp/stamps/"+seg+"/cancel", nil, nil, false, retryNever, opts)
}

// Replace requests a replacement for a stamp and returns the replacement, which has its own id
// (POST /v1/estamp/stamps/{stampId}/replace). Never retried automatically.
func (s *EstampStampsService) Replace(ctx context.Context, stampID string, opts ...RequestOption) (*EstampStamp, error) {
	seg, err := validatePathID("stampId", stampID)
	if err != nil {
		return nil, err
	}
	return callWith[EstampStamp](ctx, s.c, http.MethodPost, "/v1/estamp/stamps/"+seg+"/replace", nil, nil, false, retryNever, opts)
}

// SetPANCard sends or replaces the payer's PAN card link (POST /v1/estamp/stamps/{stampId}/pan-card).
// The API answers 202 and checks the card afterwards: read PANCard.Status from Get. Advisory, and
// never retried automatically.
func (s *EstampStampsService) SetPANCard(ctx context.Context, stampID string, p *EstampPANCardParams, opts ...RequestOption) (*EstampPANCardResult, error) {
	seg, err := validatePathID("stampId", stampID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errNilParams("SetPANCard")
	}
	if s.c.validate && (!strings.HasPrefix(p.PANCardImage, "https://") || len(p.PANCardImage) > 2048) {
		return nil, &ValidationError{Field: "panCardImage", Message: "must be an https link of at most 2048 characters"}
	}
	return callWith[EstampPANCardResult](ctx, s.c, http.MethodPost, "/v1/estamp/stamps/"+seg+"/pan-card", nil, p, false, retryNever, opts)
}

// GetCertificate returns the certificate delivery status (GET /v1/estamp/stamps/{stampId}/certificate).
// It is always a 200: check Available before using DownloadURL, which expires after about five minutes.
func (s *EstampStampsService) GetCertificate(ctx context.Context, stampID string, opts ...RequestOption) (*EstampCertificate, error) {
	seg, err := validatePathID("stampId", stampID)
	if err != nil {
		return nil, err
	}
	return call[EstampCertificate](ctx, s.c, http.MethodGet, "/v1/estamp/stamps/"+seg+"/certificate", nil, nil, false, opts)
}

// ---- wallets ----------------------------------------------------------------------------------

// WalletListParams pages a wallet's transactions or invoices. Zero values are omitted.
type WalletListParams struct {
	// Limit is the page size (at least 1; the API default is 20).
	Limit int
	// Cursor continues from a previous page's NextCursor.
	Cursor string
}

func (p *WalletListParams) query(validate bool) (url.Values, error) {
	q := url.Values{}
	if p == nil {
		return q, nil
	}
	if p.Limit != 0 {
		if validate && p.Limit < 1 {
			return nil, &ValidationError{Field: "limit", Message: "must be at least 1"}
		}
		q.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		q.Set("cursor", p.Cursor)
	}
	return q, nil
}

func checkTopup(c *Client, p *WalletTopupParams) error {
	if p == nil {
		return errNilParams("Topup")
	}
	if c.validate && (!(p.BaseAmount > 0) || math.IsInf(p.BaseAmount, 0)) {
		return &ValidationError{Field: "baseAmount", Message: "must be greater than 0"}
	}
	return nil
}

func walletList[T any](ctx context.Context, c *Client, path string, p *WalletListParams, opts []RequestOption) (*T, error) {
	q, err := p.query(c.validate)
	if err != nil {
		return nil, err
	}
	return call[T](ctx, c, http.MethodGet, path, q, nil, false, opts)
}

func walletIter[T any](p *WalletListParams, list func(ctx context.Context, p *WalletListParams) ([]T, *string, error)) *Iterator[T] {
	var base WalletListParams
	if p != nil {
		base = *p
	}
	return &Iterator[T]{cursor: base.Cursor, fetch: func(ctx context.Context, cursor string) ([]T, string, error) {
		q := base
		q.Cursor = cursor
		items, next, err := list(ctx, &q)
		return items, derefString(next), err
	}}
}

// StampDutyWalletService reads and tops up the Stamp Duty Wallet (/v1/stamp-duty-wallet), which
// pays the government duty of an order.
type StampDutyWalletService struct{ c *Client }

// GetBalance returns the wallet balance as a decimal string (GET /v1/stamp-duty-wallet).
func (s *StampDutyWalletService) GetBalance(ctx context.Context, opts ...RequestOption) (*WalletBalance, error) {
	return call[WalletBalance](ctx, s.c, http.MethodGet, "/v1/stamp-duty-wallet", nil, nil, false, opts)
}

// Topup creates a payment order to top the wallet up (POST /v1/stamp-duty-wallet/topup). The wallet
// is credited with BaseAmount; a processing fee and the GST on that fee are added on top, are not
// credited, and come back in the response (ProcessingFeeAmount, GstAmount, TotalCharged): show
// your payer those figures and never calculate them yourself, because the rate can change. A GST
// tax invoice for the fee is available from ListInvoices. Top-ups are never retried automatically.
func (s *StampDutyWalletService) Topup(ctx context.Context, p *WalletTopupParams, opts ...RequestOption) (*StampDutyWalletTopup, error) {
	if err := checkTopup(s.c, p); err != nil {
		return nil, err
	}
	return callWith[StampDutyWalletTopup](ctx, s.c, http.MethodPost, "/v1/stamp-duty-wallet/topup", nil, p, false, retryNever, opts)
}

// ListTransactions returns one page of the wallet ledger, newest first (GET /v1/stamp-duty-wallet/transactions). p may be nil.
func (s *StampDutyWalletService) ListTransactions(ctx context.Context, p *WalletListParams, opts ...RequestOption) (*WalletTransactionsPage, error) {
	return walletList[WalletTransactionsPage](ctx, s.c, "/v1/stamp-duty-wallet/transactions", p, opts)
}

// ListAllTransactions iterates over the whole ledger.
func (s *StampDutyWalletService) ListAllTransactions(p *WalletListParams, opts ...RequestOption) *Iterator[WalletTransaction] {
	return walletIter(p, func(ctx context.Context, q *WalletListParams) ([]WalletTransaction, *string, error) {
		pg, err := s.ListTransactions(ctx, q, opts...)
		if err != nil {
			return nil, nil, err
		}
		return pg.Items, pg.NextCursor, nil
	})
}

// ListInvoices returns one page of GST tax invoices for the processing fee of paid top-ups
// (GET /v1/stamp-duty-wallet/invoices). p may be nil.
func (s *StampDutyWalletService) ListInvoices(ctx context.Context, p *WalletListParams, opts ...RequestOption) (*StampDutyWalletInvoicesPage, error) {
	return walletList[StampDutyWalletInvoicesPage](ctx, s.c, "/v1/stamp-duty-wallet/invoices", p, opts)
}

// ListAllInvoices iterates over all invoices.
func (s *StampDutyWalletService) ListAllInvoices(p *WalletListParams, opts ...RequestOption) *Iterator[StampDutyWalletInvoice] {
	return walletIter(p, func(ctx context.Context, q *WalletListParams) ([]StampDutyWalletInvoice, *string, error) {
		pg, err := s.ListInvoices(ctx, q, opts...)
		if err != nil {
			return nil, nil, err
		}
		return pg.Items, pg.NextCursor, nil
	})
}

// GetInvoice returns one invoice; the id is the TopupID of the top-up (GET /v1/stamp-duty-wallet/invoices/{id}).
func (s *StampDutyWalletService) GetInvoice(ctx context.Context, id string, opts ...RequestOption) (*StampDutyWalletInvoice, error) {
	seg, err := validatePathID("id", id)
	if err != nil {
		return nil, err
	}
	return call[StampDutyWalletInvoice](ctx, s.c, http.MethodGet, "/v1/stamp-duty-wallet/invoices/"+seg, nil, nil, false, opts)
}

// ServiceFeeWalletService reads and tops up the Service Fee Wallet (/v1/service-fee-wallet), which
// pays StampMitra's service charges.
type ServiceFeeWalletService struct{ c *Client }

// GetBalance returns the wallet balance as a decimal string (GET /v1/service-fee-wallet).
func (s *ServiceFeeWalletService) GetBalance(ctx context.Context, opts ...RequestOption) (*WalletBalance, error) {
	return call[WalletBalance](ctx, s.c, http.MethodGet, "/v1/service-fee-wallet", nil, nil, false, opts)
}

// Topup creates a payment order to top the wallet up (POST /v1/service-fee-wallet/topup). The
// amounts are decimal strings read from the response; GST is added on top of BaseAmount and a GST
// invoice is issued. Top-ups are never retried automatically.
func (s *ServiceFeeWalletService) Topup(ctx context.Context, p *WalletTopupParams, opts ...RequestOption) (*ServiceFeeWalletTopup, error) {
	if err := checkTopup(s.c, p); err != nil {
		return nil, err
	}
	return callWith[ServiceFeeWalletTopup](ctx, s.c, http.MethodPost, "/v1/service-fee-wallet/topup", nil, p, false, retryNever, opts)
}

// ListTransactions returns one page of the wallet ledger, newest first (GET /v1/service-fee-wallet/transactions). p may be nil.
func (s *ServiceFeeWalletService) ListTransactions(ctx context.Context, p *WalletListParams, opts ...RequestOption) (*WalletTransactionsPage, error) {
	return walletList[WalletTransactionsPage](ctx, s.c, "/v1/service-fee-wallet/transactions", p, opts)
}

// ListAllTransactions iterates over the whole ledger.
func (s *ServiceFeeWalletService) ListAllTransactions(p *WalletListParams, opts ...RequestOption) *Iterator[WalletTransaction] {
	return walletIter(p, func(ctx context.Context, q *WalletListParams) ([]WalletTransaction, *string, error) {
		pg, err := s.ListTransactions(ctx, q, opts...)
		if err != nil {
			return nil, nil, err
		}
		return pg.Items, pg.NextCursor, nil
	})
}

// ListInvoices returns one page of GST tax invoices for top-ups (GET /v1/service-fee-wallet/invoices). p may be nil.
func (s *ServiceFeeWalletService) ListInvoices(ctx context.Context, p *WalletListParams, opts ...RequestOption) (*WalletInvoicesPage, error) {
	return walletList[WalletInvoicesPage](ctx, s.c, "/v1/service-fee-wallet/invoices", p, opts)
}

// ListAllInvoices iterates over all invoices.
func (s *ServiceFeeWalletService) ListAllInvoices(p *WalletListParams, opts ...RequestOption) *Iterator[WalletInvoice] {
	return walletIter(p, func(ctx context.Context, q *WalletListParams) ([]WalletInvoice, *string, error) {
		pg, err := s.ListInvoices(ctx, q, opts...)
		if err != nil {
			return nil, nil, err
		}
		return pg.Items, pg.NextCursor, nil
	})
}

// GetInvoice returns one invoice (GET /v1/service-fee-wallet/invoices/{id}).
func (s *ServiceFeeWalletService) GetInvoice(ctx context.Context, id string, opts ...RequestOption) (*WalletInvoice, error) {
	seg, err := validatePathID("id", id)
	if err != nil {
		return nil, err
	}
	return call[WalletInvoice](ctx, s.c, http.MethodGet, "/v1/service-fee-wallet/invoices/"+seg, nil, nil, false, opts)
}

// ---- account ----------------------------------------------------------------------------------

// AccountService reads the balance and usage of the organization that owns the key (/v1/account).
type AccountService struct{ c *Client }

// GetBalance returns the wallet balance and low-balance state (GET /v1/account/balance).
func (s *AccountService) GetBalance(ctx context.Context, opts ...RequestOption) (*AccountBalance, error) {
	return call[AccountBalance](ctx, s.c, http.MethodGet, "/v1/account/balance", nil, nil, false, opts)
}

// GetUsage returns usage for the current and previous IST month and today, with the limits (GET /v1/account/usage).
func (s *AccountService) GetUsage(ctx context.Context, opts ...RequestOption) (*AccountUsage, error) {
	return call[AccountUsage](ctx, s.c, http.MethodGet, "/v1/account/usage", nil, nil, false, opts)
}

// quoteFileName escapes a file name for a quoted Content-Disposition parameter and drops line breaks.
func quoteFileName(s string) string {
	return strings.NewReplacer("\\", "\\\\", `"`, `\"`, "\r", "", "\n", "").Replace(s)
}
