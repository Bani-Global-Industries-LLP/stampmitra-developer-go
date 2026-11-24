package stampmitra

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// specDir returns the stampmitra-developer-openapi checkout (STAMPMITRA_SPEC_DIR, default
// /home/user/stampmitra-developer-openapi), or "" when it is not present.
func specDir() string {
	dir := os.Getenv("STAMPMITRA_SPEC_DIR")
	if dir == "" {
		dir = "/home/user/stampmitra-developer-openapi"
	}
	if _, err := os.Stat(filepath.Join(dir, "openapi.json")); err != nil {
		return ""
	}
	return dir
}

// opCase is one SDK call and the HTTP request it must produce.
type opCase struct {
	Name   string
	Method string
	Path   string
	Call   func(ctx context.Context, c *Client, opts ...RequestOption) (any, error)
	// NoKey marks POSTs that must not carry an automatic Idempotency-Key (webhook replay, validate, cancel, replace, PAN-card link, top-ups).
	NoKey bool
	// Multipart marks the one POST whose body is multipart/form-data.
	Multipart bool
	// NeverRetry marks operations the API documents as x-retry-safe=false.
	NeverRetry bool
}

// wrap adapts a typed SDK call to the table's untyped signature.
func wrap[T any](f func(ctx context.Context, c *Client, opts ...RequestOption) (T, error)) func(context.Context, *Client, ...RequestOption) (any, error) {
	return func(ctx context.Context, c *Client, opts ...RequestOption) (any, error) { return f(ctx, c, opts...) }
}

// operationCases lists one call for each of the 62 operations in the spec.
func operationCases() []opCase {
	return append(legacyCases(), estampCases()...)
}

func legacyCases() []opCase {
	const svc = "estamp.instrument.rent_agreement"
	return []opCase{
		{Name: "verify.PAN", Method: "POST", Path: "/v1/verify/pan", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*PANVerification, error) {
			return c.Verify.PAN(ctx, &PANVerifyParams{PAN: "ABCDE1234F", Name: "Rahul Kumar"}, o...)
		})},
		{Name: "verify.GSTIN", Method: "POST", Path: "/v1/verify/gstin", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GSTINVerification, error) {
			return c.Verify.GSTIN(ctx, &GSTINVerifyParams{GSTIN: "27ABCDE1234F1Z5"}, o...)
		})},
		{Name: "verify.RC", Method: "POST", Path: "/v1/verify/rc", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*RCVerification, error) {
			return c.Verify.RC(ctx, &RCVerifyParams{VehicleNumber: "MH12AB1234"}, o...)
		})},
		{Name: "verify.VoterID", Method: "POST", Path: "/v1/verify/voter-id", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*VoterIDVerification, error) {
			return c.Verify.VoterID(ctx, &VoterIDVerifyParams{VoterID: "ABC1234567"}, o...)
		})},
		{Name: "verify.DrivingLicense", Method: "POST", Path: "/v1/verify/driving-license", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*DrivingLicenseVerification, error) {
			return c.Verify.DrivingLicense(ctx, &DrivingLicenseVerifyParams{DLNumber: "MH1420110012345", DOB: "1990-01-31"}, o...)
		})},
		{Name: "verify.Passport", Method: "POST", Path: "/v1/verify/passport", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*PassportVerification, error) {
			return c.Verify.Passport(ctx, &PassportVerifyParams{FileNumber: "PA1079341954215", DOB: "1990-01-31"}, o...)
		})},
		{Name: "verify.CIN", Method: "POST", Path: "/v1/verify/cin", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*CINVerification, error) {
			return c.Verify.CIN(ctx, &CINVerifyParams{CIN: "U74999DL2015PTC284009"}, o...)
		})},
		{Name: "verify.Udyam", Method: "POST", Path: "/v1/verify/udyam", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*UdyamVerification, error) {
			return c.Verify.Udyam(ctx, &UdyamVerifyParams{UdyamNumber: "UDYAM-MH-12-1234567"}, o...)
		})},
		{Name: "verify.PANToGSTIN", Method: "POST", Path: "/v1/verify/pan-to-gstin", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*PANToGSTIN, error) {
			return c.Verify.PANToGSTIN(ctx, &PANToGSTINParams{PAN: "ABCDE1234F"}, o...)
		})},
		{Name: "verify.PANToUdyam", Method: "POST", Path: "/v1/verify/pan-to-udyam", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*PANToUdyam, error) {
			return c.Verify.PANToUdyam(ctx, &PANToUdyamParams{PAN: "ABCDE1234F"}, o...)
		})},
		{Name: "verify.BankAccount", Method: "POST", Path: "/v1/verify/bank-account", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*BankAccountVerification, error) {
			return c.Verify.BankAccount(ctx, &BankAccountVerifyParams{AccountNumber: "000123456789", IFSC: "ICIC0001234", Phone: "9876543210"}, o...)
		})},
		{Name: "services.List", Method: "GET", Path: "/v1/services", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) ([]Service, error) {
			return c.Services.List(ctx, o...)
		})},
		{Name: "services.Get", Method: "GET", Path: "/v1/services/" + svc, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*Service, error) {
			return c.Services.Get(ctx, svc, o...)
		})},
		{Name: "estamp.GetSchema", Method: "GET", Path: "/v1/estamp/schema/" + svc, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Estamp.GetSchema(ctx, svc, o...)
		})},
		{Name: "estamp.ListStates", Method: "GET", Path: "/v1/estamp/states", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Estamp.ListStates(ctx, o...)
		})},
		{Name: "estamp.ListInstruments", Method: "GET", Path: "/v1/estamp/states/MH/instruments", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Estamp.ListInstruments(ctx, "MH", o...)
		})},
		{Name: "estamp.ListArticles", Method: "GET", Path: "/v1/estamp/instruments/rent_agreement/articles", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Estamp.ListArticles(ctx, "rent_agreement", o...)
		})},
		{Name: "estamp.CalculateDuty", Method: "POST", Path: "/v1/estamp/duty/calculate", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Estamp.CalculateDuty(ctx, &EstampDutyParams{StateCode: "MH", ArticleCode: "5", StampDutyAmount: 500}, o...)
		})},
		{Name: "estamp.AddParties", Method: "POST", Path: "/v1/estamp/orders/ord_123/parties", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Estamp.AddParties(ctx, "ord_123", &EstampPartiesParams{CustomerName: "A", PartyName: "B"}, o...)
		})},
		{Name: "orders.Create", Method: "POST", Path: "/v1/orders", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*OrderReference, error) {
			return c.Orders.Create(ctx, &CreateOrderParams{ExternalOrderID: "o1", ExternalCustomerID: "c1", ServiceCode: svc}, o...)
		})},
		{Name: "orders.Get", Method: "GET", Path: "/v1/orders/o1", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*OrderDetail, error) {
			return c.Orders.Get(ctx, "o1", o...)
		})},
		{Name: "orders.GetStatus", Method: "GET", Path: "/v1/orders/o1/status", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*OrderStatus, error) {
			return c.Orders.GetStatus(ctx, "o1", o...)
		})},
		{Name: "orders.GetPayment", Method: "GET", Path: "/v1/orders/o1/payment", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*OrderPayment, error) {
			return c.Orders.GetPayment(ctx, "o1", o...)
		})},
		{Name: "orders.ListDocuments", Method: "GET", Path: "/v1/orders/o1/documents", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*OrderDocuments, error) {
			return c.Orders.ListDocuments(ctx, "o1", o...)
		})},
		{Name: "orders.GetDocumentDownload", Method: "GET", Path: "/v1/orders/o1/documents/doc_1/download", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*DocumentDownloadURL, error) {
			return c.Orders.GetDocumentDownload(ctx, "o1", "doc_1", o...)
		})},
		{Name: "payments.Intents.Create", Method: "POST", Path: "/v1/payments/intents", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*PaymentIntent, error) {
			return c.Payments.Intents.Create(ctx, &CreatePaymentIntentParams{ExternalOrderID: "o1", AmountMinor: 50000, Currency: "INR", PaymentConnectionID: "pc_1"}, o...)
		})},
		{Name: "payments.Intents.Get", Method: "GET", Path: "/v1/payments/intents/pi_1", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*PaymentIntent, error) {
			return c.Payments.Intents.Get(ctx, "pi_1", o...)
		})},
		{Name: "usage.Summary", Method: "GET", Path: "/v1/usage/summary", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*UsageSummary, error) {
			return c.Usage.Summary(ctx, o...)
		})},
		{Name: "logs.List", Method: "GET", Path: "/v1/logs", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*LogsPage, error) {
			return c.Logs.List(ctx, &LogsListParams{Limit: 50}, o...)
		})},
		{Name: "webhooks.Events.List", Method: "GET", Path: "/v1/webhooks/events", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Webhooks.Events.List(ctx, &WebhookEventsListParams{Limit: 20}, o...)
		})},
		{Name: "webhooks.Events.Replay", Method: "POST", Path: "/v1/webhooks/events/evt_1/replay", NoKey: true, NeverRetry: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*GenericResponse, error) {
			return c.Webhooks.Events.Replay(ctx, "evt_1", o...)
		})},
	}
}

var testOrderParams = &EstampOrderParams{Reference: "order-1", Stamps: []EstampStampLine{{
	ClientReference: "line-001", State: "MH", Category: CategoryEStamp, ArticleCode: "ART1", Denomination: 1000,
	FirstParty: PartyDetails(EstampParty{Name: "Jane Doe", Address: "12 MG Road, Pune"}), SecondParty: ptr(PartyName("John Smith")),
}}}

func ptr[T any](v T) *T { return &v }

func estampCases() []opCase {
	return []opCase{
		{Name: "estamp.GetStatesAvailability", Method: "GET", Path: "/v1/estamp/states/availability", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampStatesAvailability, error) {
			return c.Estamp.GetStatesAvailability(ctx, &StatesAvailabilityParams{ActiveOnly: true}, o...)
		})},
		{Name: "estamp.GetState", Method: "GET", Path: "/v1/estamp/states/MH", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampState, error) {
			return c.Estamp.GetState(ctx, "MH", o...)
		})},
		{Name: "estamp.GetStateRequirements", Method: "GET", Path: "/v1/estamp/states/MH/requirements", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampRequirements, error) {
			return c.Estamp.GetStateRequirements(ctx, "MH", &StateRequirementsParams{ArticleCode: "ART1", Denomination: 1000, Category: CategoryEStamp}, o...)
		})},
		{Name: "estamp.ListDistricts", Method: "GET", Path: "/v1/estamp/states/MH/districts", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) ([]EstampDistrict, error) {
			return c.Estamp.ListDistricts(ctx, "MH", o...)
		})},
		{Name: "estamp.ListSROs", Method: "GET", Path: "/v1/estamp/states/MH/districts/PUN/sros", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) ([]EstampSRO, error) {
			return c.Estamp.ListSROs(ctx, "MH", "PUN", o...)
		})},
		{Name: "estamp.Orders.Validate", Method: "POST", Path: "/v1/estamp/orders/validate", NoKey: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampValidateResult, error) {
			return c.Estamp.Orders.Validate(ctx, testOrderParams, o...)
		})},
		{Name: "estamp.Orders.Create", Method: "POST", Path: "/v1/estamp/orders", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampOrder, error) {
			return c.Estamp.Orders.Create(ctx, testOrderParams, o...)
		})},
		{Name: "estamp.Orders.List", Method: "GET", Path: "/v1/estamp/orders", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampOrderList, error) {
			return c.Estamp.Orders.List(ctx, &EstampOrdersListParams{Limit: 20}, o...)
		})},
		{Name: "estamp.Orders.Get", Method: "GET", Path: "/v1/estamp/orders/estord_1", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampOrder, error) {
			return c.Estamp.Orders.Get(ctx, "estord_1", o...)
		})},
		{Name: "estamp.Orders.Cancel", Method: "POST", Path: "/v1/estamp/orders/estord_1/cancel", NoKey: true, NeverRetry: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampOrder, error) {
			return c.Estamp.Orders.Cancel(ctx, "estord_1", o...)
		})},
		{Name: "estamp.Orders.Export", Method: "GET", Path: "/v1/estamp/orders/estord_1/export", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampOrderExport, error) {
			return c.Estamp.Orders.Export(ctx, "estord_1", o...)
		})},
		{Name: "estamp.Orders.ListStamps", Method: "GET", Path: "/v1/estamp/orders/estord_1/stamps", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampStampList, error) {
			return c.Estamp.Orders.ListStamps(ctx, "estord_1", o...)
		})},
		{Name: "estamp.Orders.UploadDocument", Method: "POST", Path: "/v1/estamp/orders/estord_1/documents", Multipart: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampDocument, error) {
			return c.Estamp.Orders.UploadDocument(ctx, "estord_1", &EstampDocumentUploadParams{File: strings.NewReader(testPDF), FileName: "pan.pdf", Type: DocumentTypePANCard}, o...)
		})},
		{Name: "estamp.Stamps.Get", Method: "GET", Path: "/v1/estamp/stamps/eststamp_1", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampStamp, error) {
			return c.Estamp.Stamps.Get(ctx, "eststamp_1", o...)
		})},
		{Name: "estamp.Stamps.Cancel", Method: "POST", Path: "/v1/estamp/stamps/eststamp_1/cancel", NoKey: true, NeverRetry: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampStamp, error) {
			return c.Estamp.Stamps.Cancel(ctx, "eststamp_1", o...)
		})},
		{Name: "estamp.Stamps.Replace", Method: "POST", Path: "/v1/estamp/stamps/eststamp_1/replace", NoKey: true, NeverRetry: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampStamp, error) {
			return c.Estamp.Stamps.Replace(ctx, "eststamp_1", o...)
		})},
		{Name: "estamp.Stamps.SetPANCard", Method: "POST", Path: "/v1/estamp/stamps/eststamp_1/pan-card", NoKey: true, NeverRetry: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampPANCardResult, error) {
			return c.Estamp.Stamps.SetPANCard(ctx, "eststamp_1", &EstampPANCardParams{PANCardImage: "https://cdn.example.com/kyc/pan-4417.jpg"}, o...)
		})},
		{Name: "estamp.Stamps.GetCertificate", Method: "GET", Path: "/v1/estamp/stamps/eststamp_1/certificate", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampCertificate, error) {
			return c.Estamp.Stamps.GetCertificate(ctx, "eststamp_1", o...)
		})},
		{Name: "estamp.GetWallet", Method: "GET", Path: "/v1/estamp/wallet", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*EstampWallet, error) {
			return c.Estamp.GetWallet(ctx, o...)
		})},
		{Name: "stampDutyWallet.GetBalance", Method: "GET", Path: "/v1/stamp-duty-wallet", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*WalletBalance, error) {
			return c.StampDutyWallet.GetBalance(ctx, o...)
		})},
		{Name: "stampDutyWallet.Topup", Method: "POST", Path: "/v1/stamp-duty-wallet/topup", NoKey: true, NeverRetry: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*StampDutyWalletTopup, error) {
			return c.StampDutyWallet.Topup(ctx, &WalletTopupParams{BaseAmount: 1000}, o...)
		})},
		{Name: "stampDutyWallet.ListTransactions", Method: "GET", Path: "/v1/stamp-duty-wallet/transactions", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*WalletTransactionsPage, error) {
			return c.StampDutyWallet.ListTransactions(ctx, &WalletListParams{Limit: 20}, o...)
		})},
		{Name: "stampDutyWallet.ListInvoices", Method: "GET", Path: "/v1/stamp-duty-wallet/invoices", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*StampDutyWalletInvoicesPage, error) {
			return c.StampDutyWallet.ListInvoices(ctx, &WalletListParams{Limit: 20}, o...)
		})},
		{Name: "stampDutyWallet.GetInvoice", Method: "GET", Path: "/v1/stamp-duty-wallet/invoices/inv_1", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*StampDutyWalletInvoice, error) {
			return c.StampDutyWallet.GetInvoice(ctx, "inv_1", o...)
		})},
		{Name: "serviceFeeWallet.GetBalance", Method: "GET", Path: "/v1/service-fee-wallet", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*WalletBalance, error) {
			return c.ServiceFeeWallet.GetBalance(ctx, o...)
		})},
		{Name: "serviceFeeWallet.Topup", Method: "POST", Path: "/v1/service-fee-wallet/topup", NoKey: true, NeverRetry: true, Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*ServiceFeeWalletTopup, error) {
			return c.ServiceFeeWallet.Topup(ctx, &WalletTopupParams{BaseAmount: 500}, o...)
		})},
		{Name: "serviceFeeWallet.ListTransactions", Method: "GET", Path: "/v1/service-fee-wallet/transactions", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*WalletTransactionsPage, error) {
			return c.ServiceFeeWallet.ListTransactions(ctx, &WalletListParams{Limit: 20}, o...)
		})},
		{Name: "serviceFeeWallet.ListInvoices", Method: "GET", Path: "/v1/service-fee-wallet/invoices", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*WalletInvoicesPage, error) {
			return c.ServiceFeeWallet.ListInvoices(ctx, &WalletListParams{Limit: 20}, o...)
		})},
		{Name: "serviceFeeWallet.GetInvoice", Method: "GET", Path: "/v1/service-fee-wallet/invoices/inv_1", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*WalletInvoice, error) {
			return c.ServiceFeeWallet.GetInvoice(ctx, "inv_1", o...)
		})},
		{Name: "account.GetBalance", Method: "GET", Path: "/v1/account/balance", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*AccountBalance, error) {
			return c.Account.GetBalance(ctx, o...)
		})},
		{Name: "account.GetUsage", Method: "GET", Path: "/v1/account/usage", Call: wrap(func(ctx context.Context, c *Client, o ...RequestOption) (*AccountUsage, error) {
			return c.Account.GetUsage(ctx, o...)
		})},
	}
}

const testPDF = "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"
