# StampMitra Go SDK

[![CI](https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Bani-Global-Industries-LLP/stampmitra-developer-go.svg)](https://pkg.go.dev/github.com/Bani-Global-Industries-LLP/stampmitra-developer-go)
[![Go version](https://img.shields.io/badge/go-1.21%2B-00ADD8)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

> The pkg.go.dev badge shows documentation only once a version tag is published. The `v0.2.1` tag is created at release, so
> the `go get` line below works only after the release.

Official Go client for the [StampMitra Developer API](https://developer.stampmitra.in/docs):
Indian identity verification (PAN, GSTIN, vehicle RC, Voter ID, driving licence, passport, CIN, Udyam, bank account)
and e-Stamp Paper. OpenAPI spec: <https://developer.stampmitra.in/openapi.json>.

> **Status: pre-1.0 (0.2.1).** The API surface may change in minor releases. Pin your version.

SDK guide: <https://developer.stampmitra.in/sdks/go> | API docs: <https://developer.stampmitra.in/docs>

## Install

```bash
go get github.com/Bani-Global-Industries-LLP/stampmitra-developer-go@v0.2.1
```

The `v0.2.1` tag is created at release: until then this line does not resolve.

Requires Go 1.21 or newer (CI runs 1.21, 1.22, 1.23 and 1.24). No third-party dependencies: the standard library only.

> **The module path is case-sensitive.** Import it exactly as `github.com/Bani-Global-Industries-LLP/stampmitra-developer-go`
> (capital letters, like the GitHub organisation). The package name is `stampmitra`.

## Quickstart

```go
package main

import (
	"context"
	"fmt"
	"log"

	stampmitra "github.com/Bani-Global-Industries-LLP/stampmitra-developer-go"
)

func main() {
	client, err := stampmitra.NewClient("") // empty = read STAMPMITRA_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	pan, err := client.Verify.PAN(context.Background(), &stampmitra.PANVerifyParams{PAN: "ABCDE1234F", Name: "Rahul Kumar"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(pan.Valid, pan.NameMatchResult, pan.ReferenceID)
}
```

## Authentication

Keys look like `sk_test_...` (free sandbox) or `sk_live_...` (metered). `NewClient("")` reads the
`STAMPMITRA_API_KEY` environment variable. Keys that do not match `^sk_(test|live)_[A-Za-z0-9_-]{8,}$` are rejected unless you
pass `WithAllowCustomKey()`. `client.Environment` is `"test"`, `"live"` or `"custom"`.

> **Security:** server-side only. Never ship `sk_` keys in mobile apps, browser code or public repositories.

Client options (`stampmitra.NewClient(key, opts...)`):

| Option | Default | Meaning |
|---|---|---|
| `WithBaseURL(u)` | `https://api-b2b.stampmitra.in` | Must be `https` (plain `http` only for `localhost`, `127.0.0.1`, `::1`), no credentials, no query |
| `WithTimeout(d)` | 30s | Per attempt |
| `WithMaxRetries(n)` | 2 | Automatic retries (so at most 3 attempts) |
| `WithBackoff(base, max)` | 500ms, 8s | Full-jitter exponential backoff |
| `WithHTTPClient(c)` | `http.Client{}` | Your transport/proxy settings are kept; redirects are still never followed |
| `WithDefaultHeader(k, v)` | none | Sent on every request |
| `WithAppInfo(AppInfo{...})` | none | Added to the User-Agent |
| `WithLogger(*slog.Logger)` | off | Redacted debug logging |
| `WithValidateInput(bool)` | on | Client-side validation |
| `WithAllowCustomKey()` | off | Accept non-`sk_` keys |

Every method takes a `context.Context` first and accepts per-call options last: `WithRequestTimeout`, `WithRequestMaxRetries`,
`WithIdempotencyKey`, `WithRequestHeader`, `WithRawResponse`.

## Resources

| Area | Methods |
|---|---|
| `client.Verify` | `PAN`, `GSTIN`, `RC`, `VoterID`, `DrivingLicense`, `Passport`, `CIN`, `Udyam`, `PANToGSTIN`, `PANToUdyam`, `BankAccount` |
| `client.Services` | `List`, `Get` |
| `client.Estamp` | `GetSchema`, `ListStates`, `GetStatesAvailability`, `GetState`, `GetStateRequirements`, `ListDistricts`, `ListSROs`, `ListInstruments`, `ListArticles`, `CalculateDuty`, `AddParties`, `GetWallet` |
| `client.Estamp.Orders` | `Validate`, `Create`, `List`, `ListAll` (iterator), `Get`, `Cancel`, `Export`, `ListStamps`, `UploadDocument` |
| `client.Estamp.Stamps` | `Get`, `Cancel`, `Replace`, `SetPANCard`, `GetCertificate` |
| `client.StampDutyWallet` (Stamp duty wallet), `client.ServiceFeeWallet` (Service wallet; name kept for compatibility) | `GetBalance`, `Topup`, `ListTransactions`, `ListAllTransactions`, `ListInvoices`, `ListAllInvoices`, `GetInvoice` |
| `client.Account` | `GetBalance`, `GetUsage` |
| `client.Orders` | `Create`, `Get`, `GetStatus`, `GetPayment`, `ListDocuments`, `GetDocumentDownload` |
| `client.Payments.Intents` | `Create`, `Get` |
| `client.Usage` | `Summary` |
| `client.Logs` | `List`, `ListAll` (iterator) |
| `client.Webhooks.Events` | `List`, `Replay` |

```go
ctx := context.Background()

// Verification (every call is billed, including valid=false results)
pan, err := client.Verify.PAN(ctx, &stampmitra.PANVerifyParams{PAN: "ABCDE1234F", Name: "Rahul Kumar"})
gst, err := client.Verify.GSTIN(ctx, &stampmitra.GSTINVerifyParams{GSTIN: "27ABCDE1234F1Z5"})
rc, err := client.Verify.RC(ctx, &stampmitra.RCVerifyParams{VehicleNumber: "MH12AB1234"})
voter, err := client.Verify.VoterID(ctx, &stampmitra.VoterIDVerifyParams{VoterID: "ABC1234567"})
dl, err := client.Verify.DrivingLicense(ctx, &stampmitra.DrivingLicenseVerifyParams{DLNumber: "MH1420110012345", DOB: "1990-01-31"}) // IP-allowlisted key
pp, err := client.Verify.Passport(ctx, &stampmitra.PassportVerifyParams{FileNumber: "PA1079341954215", DOB: "1990-01-31"})        // IP-allowlisted key
cin, err := client.Verify.CIN(ctx, &stampmitra.CINVerifyParams{CIN: "U74999DL2015PTC284009"})
udyam, err := client.Verify.Udyam(ctx, &stampmitra.UdyamVerifyParams{UdyamNumber: "UDYAM-MH-12-1234567"})
gstins, err := client.Verify.PANToGSTIN(ctx, &stampmitra.PANToGSTINParams{PAN: "ABCDE1234F"})
udyams, err := client.Verify.PANToUdyam(ctx, &stampmitra.PANToUdyamParams{PAN: "ABCDE1234F"})
bank, err := client.Verify.BankAccount(ctx, &stampmitra.BankAccountVerifyParams{AccountNumber: "000123456789", IFSC: "ICIC0001234", Name: "Rahul Kumar"})

// Services and e-Stamp reference data
services, err := client.Services.List(ctx) // []stampmitra.Service
service, err := client.Services.Get(ctx, "estamp.instrument.rent_agreement")
schema, err := client.Estamp.GetSchema(ctx, "estamp.instrument.rent_agreement") // *GenericResponse
states, err := client.Estamp.ListStates(ctx)
instruments, err := client.Estamp.ListInstruments(ctx, "MH")
articles, err := client.Estamp.ListArticles(ctx, "rent_agreement")
duty, err := client.Estamp.CalculateDuty(ctx, &stampmitra.EstampDutyParams{StateCode: "MH", ArticleCode: "5", StampDutyAmount: 500})
parties, err := client.Estamp.AddParties(ctx, "ord_ext_123", &stampmitra.EstampPartiesParams{CustomerName: "A", PartyName: "B"})

// Orders and payments: orders are addressed by YOUR externalOrderId
order, err := client.Orders.Create(ctx, &stampmitra.CreateOrderParams{
	ExternalOrderID: "ord_ext_123", ExternalCustomerID: "cust_ext_456", ServiceCode: "estamp.instrument.rent_agreement",
}, stampmitra.WithIdempotencyKey("create-ord_ext_123"))
detail, err := client.Orders.Get(ctx, order.ExternalOrderID)
status, err := client.Orders.GetStatus(ctx, order.ExternalOrderID)
payment, err := client.Orders.GetPayment(ctx, order.ExternalOrderID)
docs, err := client.Orders.ListDocuments(ctx, order.ExternalOrderID)
link, err := client.Orders.GetDocumentDownload(ctx, order.ExternalOrderID, "doc_1") // link.DownloadURL is short-lived
intent, err := client.Payments.Intents.Create(ctx, &stampmitra.CreatePaymentIntentParams{ExternalOrderID: "ord_ext_123", AmountMinor: 50000, Currency: "INR", PaymentConnectionID: "pc_1"})
intent, err = client.Payments.Intents.Get(ctx, intent.ID)

// Usage, logs, webhook events
usage, err := client.Usage.Summary(ctx)
page, err := client.Logs.List(ctx, &stampmitra.LogsListParams{Service: "pan.verify", Status: "success", Limit: 50})
events, err := client.Webhooks.Events.List(ctx, &stampmitra.WebhookEventsListParams{Limit: 20})
replay, err := client.Webhooks.Events.Replay(ctx, "evt_1")
```

Create returns `externalOrderId` (there is no `id`); pass it to `Get`, `GetStatus`, `GetPayment`, `ListDocuments`,
`GetDocumentDownload` and `Estamp.AddParties`.

### e-Stamp orders

An order is one state and up to 5000 stamp lines (`Quantity` up to 1000 per line). The flow is validate, create, optionally
upload the payer's PAN card, then read the stamps and their certificates. Order ids here are the ids the API returns
(`estord_...`), not your `externalOrderId`.

```go
second := stampmitra.PartyName("John Smith") // SecondParty is a *Party: nil when the article has one party
order := &stampmitra.EstampOrderParams{
	Reference: "order-2026-0917", // your own reference; distinct from the Idempotency-Key
	Stamps: []stampmitra.EstampStampLine{{
		ClientReference: "line-001",
		State:           "MH",
		Category:        stampmitra.CategoryEStamp,
		ArticleCode:     "ART1",
		Denomination:    1000,
		// A party is a bare name or an object; use the object when the state wants an address
		// or a father's or husband's name (see GetStateRequirements).
		FirstParty:  stampmitra.PartyDetails(stampmitra.EstampParty{Name: "Jane Doe", Address: "12 MG Road, Pune"}),
		SecondParty: &second,
	}},
}

// 1. Validate: nothing is saved or charged. Show Warnings and Quote to your user.
check, err := client.Estamp.Orders.Validate(ctx, order)
if err == nil && !check.Valid {
	log.Println("missing details:", check.Errors)
}

// 2. Create: debits the Stamp duty wallet (duty) and the Service wallet (fees, before GST). An Idempotency-Key is required by the API; when you pass none the SDK
//    generates one and re-uses it on every automatic retry. Derive it from your own reference if your program may re-run.
created, err := client.Estamp.Orders.Create(ctx, order, stampmitra.WithIdempotencyKey("create-order-2026-0917"))

// 3. Upload the payer's PAN card: PDF, JPEG or PNG, at most 10 MB (optional).
f, _ := os.Open("pan-card.pdf")
defer f.Close()
doc, err := client.Estamp.Orders.UploadDocument(ctx, created.ID, &stampmitra.EstampDocumentUploadParams{
	File: f, FileName: "pan-card.pdf", Type: stampmitra.DocumentTypePANCard,
})

// 4. List the stamps, then 5. fetch a certificate (always HTTP 200: check Available).
stamps, err := client.Estamp.Orders.ListStamps(ctx, created.ID)
cert, err := client.Estamp.Stamps.GetCertificate(ctx, stamps.Items[0].ID)
if err == nil && cert.Available {
	log.Println("download before", cert.ExpiresAt, cert.DownloadURL) // the link is short-lived
}
```

- The **PAN card**: you can place an order without one, but a PAN card is **mandatory to complete the stamp** (the SDK's validation is
  unchanged; the API may flag a missing card with the `PAN_CARD_RECOMMENDED` advisory). Send `PANCardImage` (an `https` link) on a stamp line,
  upload the file as above, or send or replace a link later with `Estamp.Stamps.SetPANCard` (the API answers 202 and checks the card
  afterwards: read `PANCard.Status` from `Stamps.Get`).
- `FirstParty` and `SecondParty` are a `Party`: `PartyName("Jane Doe")` is sent as a string, `PartyDetails(EstampParty{...})` as an object
  (`Name`, `Address`, `Phone`, `FatherHusbandName`, `Gender`). Both forms decode back into a `Party`.
  `PropertyArea` is `&EstampPropertyArea{Unit: stampmitra.AreaSquareFeet, Value: 850}`.
- `Cancel` (order and stamp), `Replace`, `SetPANCard` and the wallet top-ups are never retried automatically; see below.
- A rejected body is a 400 `*APIError` whose `FieldErrors` list every finding (`Field`, `Code`, `Message`, `Index`); a 402 names the short
  wallet in `Wallet` (`stampmitra.WalletStampDuty` for the Stamp duty wallet or `stampmitra.WalletServiceFee` for the Service wallet). Top that wallet up and send a new request.

```go
var order *stampmitra.EstampOrderParams // built as above
_, err := client.Estamp.Orders.Create(ctx, order)
var apiErr *stampmitra.APIError
if errors.As(err, &apiErr) {
	for _, f := range apiErr.FieldErrors {
		log.Println(f.Field, f.Code, f.Message)
	}
	if stampmitra.IsPaymentRequired(err) {
		log.Println("top up the", apiErr.Wallet, "wallet")
	}
}
```

### Wallets

Two prepaid wallets back your account:

- The **Service wallet** is one balance. It pays verification calls and the e-Stamp fees (procurement charge, API charge and
  convenience fee). Top it up by at least ₹1,000 plus 18% GST. GST on fees is paid at top-up, so an order debits the fees before GST.
- The **Stamp duty wallet** holds the statutory stamp duty that orders debit.

The Service wallet is `client.ServiceFeeWallet` (the name is kept for compatibility) and the Stamp duty wallet is
`client.StampDutyWallet`. In API responses and errors the wallet names stay `SERVICE_FEE` and `STAMP_DUTY` (for example
`details.wallet`). There is no signup credit for new accounts. Balances are decimal strings.

```go
sd, err := client.StampDutyWallet.GetBalance(ctx)
top, err := client.StampDutyWallet.Topup(ctx, &stampmitra.WalletTopupParams{BaseAmount: 1000})
fmt.Println(top.ProcessingFeePercent, top.ProcessingFeeAmount, top.GstAmount, top.TotalCharged, top.CreditAmount)
invoices, err := client.StampDutyWallet.ListInvoices(ctx, nil)
both, err := client.Estamp.GetWallet(ctx) // both wallets in one read
```

A Stamp duty wallet top-up adds a **6% processing fee plus 18% GST on that fee** on top of `BaseAmount`; the wallet is credited the amount
you added (`CreditAmount`, equal to `BaseAmount`), and stamp duty itself carries no GST. A GST tax invoice for the fee is available
from `ListInvoices` / `GetInvoice` (the invoice id is the top-up's `TopupID`). Show your payer the figures the response returns
(`ProcessingFeeAmount`, `GstAmount`, `TotalCharged`): the SDK never computes fees, because the rate can change. In that response
`BaseAmount`, `GstAmount` and `TotalCharged` are JSON numbers (`float64`) and the other amounts are strings. A Service wallet top-up
returns every amount as a string. Both top-ups return a payment order to complete in checkout (live keys); a sandbox top-up adds
fake credit at once.

### Models

Responses are structs with `json` tags (`pan.RegisteredName`). Every response struct has an `Extra map[string]json.RawMessage` holding
fields the SDK version does not model, and `json.Marshal` writes them back, so nothing the API sends is lost. Optional fields
are plain values (zero when absent) except nullable ones, which are pointers (`Service.Category *string`). Endpoints whose
shape the API does not fix (e-Stamp reference data, duty, parties, webhook event lists and replays) return `*GenericResponse`:
`Raw()`, `Decode(&v)`, `Get(key)`, `Map()`, `Items()`, `NextCursor()`. Upstream-owned nested values (`OrderDetail.Order`,
`OrderStatus.Status`, ...) are `json.RawMessage`.

### Raw HTTP metadata

```go
var raw stampmitra.Response
st, err := client.Orders.GetStatus(ctx, "ord_ext_123", stampmitra.WithRawResponse(&raw))
fmt.Println(st != nil, err == nil, raw.StatusCode, raw.RequestID, raw.Attempts, raw.Header.Get("Content-Type"))
```

## Errors

Every non-2xx response is an `*stampmitra.APIError` (`Status`, `Code`, `Message`, `RequestID`, `Field`, `RetryAfter`, `Body`,
`Attempts`, `FieldErrors`, `Wallet`, `Details`). Classify with `errors.Is` and the sentinels, or the helpers:

| HTTP | Sentinel | Helper |
|---|---|---|
| 400, 422 (and local validation) | `ErrValidation` (`FieldErrors` lists each rejected input) | `IsValidation` |
| 401 | `ErrAuthentication` | `IsAuthentication` |
| 402 | `ErrPaymentRequired` (`Wallet` names the short wallet on e-Stamp orders) | `IsPaymentRequired` |
| 403 | `ErrPermission` (server message verbatim, e.g. IP allowlist) | `IsPermission` |
| 404 | `ErrNotFound` | `IsNotFound` |
| 409 | `ErrConflict` | `IsConflict` |
| 429 | `ErrRateLimit` (`RetryAfter` when the header is sent) | `IsRateLimit` |
| 5xx | `ErrUpstream` | `IsUpstream` |
| network / timeout | `*ConnectionError`: `ErrConnection`, `ErrTimeout` | `IsConnection`, `IsTimeout` |

Other statuses are still `*APIError` without a sentinel; non-JSON error bodies map by status. A cancelled or expired context
returns the context's own error (`errors.Is(err, context.Canceled)`).

```go
_, err := client.Verify.GSTIN(ctx, &stampmitra.GSTINVerifyParams{GSTIN: "27ABCDE1234F1Z5"})
var apiErr *stampmitra.APIError
switch {
case err == nil:
	// success
case errors.As(err, &apiErr) && stampmitra.IsRateLimit(err):
	log.Println("rate limited; retry after", apiErr.RetryAfter) // zero when the API sent no Retry-After
case errors.As(err, &apiErr):
	log.Printf("%d %s (request %s): %s", apiErr.Status, apiErr.Code, apiErr.RequestID, apiErr.Message)
case stampmitra.IsValidation(err):
	log.Println("invalid input, nothing was sent:", err)
default:
	log.Println("request failed:", err)
}
```

## Retries and idempotency

Default `WithMaxRetries(2)` with exponential backoff and full jitter (500ms base, 8s cap); a `Retry-After` header (seconds or HTTP date,
capped at 60s) overrides the computed delay. The API sends it on every 429 and 503 and on `409 IDEMPOTENCY_IN_PROGRESS`.
Retried: 429, 502, 503, 504, connection errors and timeouts, plus a 409 `IDEMPOTENCY_IN_PROGRESS` on a request that carries an
Idempotency-Key (the identical request is sent again after `Retry-After`). Not retried: other statuses (400, 401, 402, 403, 404,
422, 500, any other 409 ...). GET is always retry-safe.

The API honours `Idempotency-Key` on metered calls, order creation and document upload, so the SDK generates a random UUIDv4 key (from
`crypto/rand`; it fails rather than fall back to a weak generator) for **every POST that accepts one** and reuses the **same key
across automatic retries**. A retried verification, order or payment intent therefore replays the original result instead of being
billed or created twice. `Estamp.Orders.Create` requires a key: when you pass none, the SDK sends one. Pass your own with
`WithIdempotencyKey` (use one derived from your own order id when your program itself may re-run).

These calls are **never retried automatically** (the API marks them not retry-safe and they take no Idempotency-Key, so the SDK
sends none, even if you pass `WithIdempotencyKey`): `Estamp.Orders.Cancel`, `Estamp.Stamps.Cancel`, `Estamp.Stamps.Replace`,
`Estamp.Stamps.SetPANCard`, `StampDutyWallet.Topup`, `ServiceFeeWallet.Topup` and `Webhooks.Events.Replay`. After an error, read
the resource back before trying again. `Estamp.Orders.Validate` is a dry run (nothing created or billed, no key) and is retried like a GET.

## Pagination

```go
it := client.Logs.ListAll(&stampmitra.LogsListParams{Service: "pan.verify", Limit: 100})
for it.Next(ctx) {
	entry := it.Log()
	fmt.Println(entry.ID, entry.CreatedAt)
}
if err := it.Err(); err != nil {
	log.Fatal(err)
}
```

`Estamp.Orders.ListAll`, `StampDutyWallet.ListAllTransactions`, `ListAllInvoices` and the `ServiceFeeWallet` equivalents return a
generic `*Iterator[T]` (`Next(ctx)`, `Item()`, `Err()`) that works the same way.

`ListAll` performs no I/O itself; `Next` fetches pages lazily and follows `nextCursor`. For one page use `Logs.List` (`page.Items`,
`page.NextCursor`). `Webhooks.Events.List` returns a `*GenericResponse` with `Items()` and `NextCursor()`.

## Webhooks

Deliveries carry `X-Webhook-Signature`, `X-Webhook-Timestamp` (unix seconds) and `X-Webhook-Event-Id`. The signature is
`hex(HMAC-SHA256(secret, "{timestamp}.{rawBody}"))` over the body **exactly as received**. Verification lives in this package (no
sub-package): `VerifySignature`, `ConstructEvent`, `ConstructEventFromHeaders`.

```go
http.HandleFunc("/webhooks/stampmitra", func(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20)) // raw bytes: do not decode and re-encode
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ev, err := stampmitra.ConstructEventFromHeaders(body, r.Header, secret)
	if err != nil {
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}
	switch e := ev.(type) {
	case *stampmitra.OrderDocumentReadyEvent:
		log.Println("document ready for", e.CoreOrderID, e.ID)
	case *stampmitra.UnknownEvent:
		log.Println("unrecognised event", e.RawType)
	}
	w.WriteHeader(http.StatusNoContent)
})
```

The eight `order.*` events decode to typed values (`*OrderCreatedEvent`, `*OrderPaymentConfirmedEvent`, `*OrderFulfilmentStartedEvent`,
`*OrderFulfilmentCompletedEvent`, `*OrderFailedEvent`, `*OrderCancelledEvent`, `*OrderRefundCompletedEvent`, `*OrderDocumentReadyEvent`), all
embedding `OrderEvent` (`CoreOrderID`, `CoreEventType`, `OccurredAt`, `ID`, and `Extra` for every other field); anything else is an
`*UnknownEvent`. The comparison is constant-time; timestamps more than 300s (`WithTolerance`) from now are rejected; an empty secret is
rejected; failures return `*SignatureVerificationError` (`Reason`: `invalid_timestamp`, `timestamp_outside_tolerance`,
`signature_mismatch`, ...), which never contains the secret. For tests, `SignPayload` computes a signature. A runnable receiver is in
[`examples/webhook_receiver`](examples/webhook_receiver/main.go).

## Validation

Inputs are checked before sending (PAN, GSTIN, IFSC, CIN, Udyam, vehicle number, 9-18 digit account number, passport file number,
driving licence, voter ID, phone, ISO dates, length limits). Values are trimmed and ASCII upper-cased (bank account numbers: trimmed only);
spaces and hyphens inside identifiers are rejected, never silently removed. A failure returns a `*ValidationError` (matches
`ErrValidation`) with no network call and no billing; its message never echoes the value. The validators are also exported
(`ValidatePAN`, `ValidateGSTIN`, `ValidateIFSC`, `ValidateCIN`, `ValidateUdyam`, `ValidateVehicleNumber`, `ValidateBankAccount`,
`ValidatePassportFileNumber`, `ValidateDrivingLicense`, `ValidateVoterID`). Turn all of it off with `WithValidateInput(false)`.

## Logging

`WithLogger(slog.New(...))` logs requests, responses and retries at `slog.LevelDebug`. `Authorization` and `X-API-Key` are redacted, and
identifiers (PAN, GSTIN, account numbers, ...) and personal data (names, dates of birth, addresses) in bodies are masked. Nothing is logged by default.

## Pricing

The SDK is free and MIT-licensed. You pay only for successful API calls, from your prepaid Service wallet. There is no signup credit. Sandbox is free: mock data on every endpoint, and it never debits your Service wallet.

Per-call rates by plan, in rupees, excluding 18% GST:

| Product | Pay-as-you-go | Growth | Scale | Startup | Enterprise |
|---|---|---|---|---|---|
| Standard verification (PAN, GSTIN, Voter ID, Driving Licence, CIN, Udyam, cross-lookups) | ₹5 | ₹4 | ₹3.50 | ₹4 | Custom |
| Passport and Vehicle RC | ₹9 | ₹8.50 | ₹8 | ₹8.50 | Custom |
| Bank account (penny-drop) | ₹4 | ₹4 | ₹4 | ₹4 | Custom |
| e-Stamp API fee (StampMitra's charge only) | ₹5 | ₹4 | ₹3 | ₹4 | Custom |

| | Sandbox | Pay-as-you-go | Growth | Scale | Startup | Enterprise |
|---|---|---|---|---|---|---|
| Plan fee | Free | ₹0 / month | ₹999 / month | ₹4,999 / month | Free for 6 months, then 50% off for 6 | Custom |
| Seats / API keys / webhooks | 1 / 2 / 1 | 3 / 5 / 3 | 10 / 20 / 10 | 25 / 50 / 25 | 10 / 20 / 10 | Custom |
| Requests per minute | 20 | 60 | 300 | 1,000 | 300 | Custom |
| Log retention | 7 days | 30 days | 90 days | 365 days | 90 days | Custom |
| IP allowlisting | No | No | Yes | Yes | Yes | Yes |
| White-label and custom domain | No | No | No | Yes | No | Yes |

Growth and Scale are also available yearly (₹9,990 and ₹49,990, two months free). There are separate Startup and NGO programmes. Plans, rates and limits can change, and [developer.stampmitra.in/pricing](https://developer.stampmitra.in/pricing) is the source of truth; see it for how an e-Stamp order is priced in total. API e-Stamp orders are priced per paper: ₹99 procurement charge, plus ₹5 API charge, plus a convenience fee (by default 5% of that paper's duty above ₹1,000). The duty is multiplied by the quantity. Sandbox quotes use the same default rates as live; rates agreed for your account are not applied in sandbox. The fees are debited from the Service wallet before GST (GST was paid when you topped up), and the duty from the Stamp duty wallet. See the pricing page for current rates.

## Security

- Use keys server-side only and store them in environment variables or a secret manager; never commit them.
- TLS verification is never disabled. Redirects are never followed (so credentials cannot leak to another host), and the base URL must
  be `https://` except for localhost.
- The API key never appears in `fmt` output of the client (`%v`, `%+v`, `%#v`), errors or logs; the webhook secret never appears in errors.
- Always verify webhook signatures on the raw body and keep the default 300s timestamp tolerance.
- No third-party dependencies, so `govulncheck` findings come only from the Go standard library: build with a patched Go release.
- Report vulnerabilities as described in [SECURITY.md](SECURITY.md), not in public issues.

## Supported versions

Go 1.21, 1.22, 1.23, 1.24 (CI matrix; the module declares `go 1.21`). Only the latest minor release receives fixes while the SDK is
pre-1.0. See [CHANGELOG.md](CHANGELOG.md).

## Development

```bash
make check         # gofmt, vet, staticcheck, golangci-lint, go test -race -cover, spec coverage
make conformance   # needs Node + a checkout of stampmitra-developer-openapi (STAMPMITRA_SPEC_DIR)
make examples      # runs every program in examples/ against the mock server
```

`spec/openapi.json` is a snapshot of the public spec; `TestSpecCoverage` (`make coverage-spec`) fails if any `METHOD + path` in it has no
SDK method. See [CONTRIBUTING.md](CONTRIBUTING.md) and [RELEASING.md](RELEASING.md). Runnable examples: [`examples/`](examples/)
(a separate module so it does not add anything to your dependency graph).

Licensed under the [MIT License](LICENSE). Copyright Bani Global Industries LLP (StampMitra).
