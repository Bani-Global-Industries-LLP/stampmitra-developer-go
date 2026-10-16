# StampMitra Go SDK

[![CI](https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/actions/workflows/ci.yml/badge.svg)](https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Bani-Global-Industries-LLP/stampmitra-developer-go.svg)](https://pkg.go.dev/github.com/Bani-Global-Industries-LLP/stampmitra-developer-go)
[![Go version](https://img.shields.io/badge/go-1.21%2B-00ADD8)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

> The pkg.go.dev badge shows documentation only once the first version tag (v0.1.0) is published.

Official Go client for the [StampMitra Developer API](https://developer.stampmitra.in/docs):
Indian identity verification (PAN, GSTIN, vehicle RC, Voter ID, driving licence, passport, CIN, Udyam, bank account)
and e-Stamp Paper. OpenAPI spec: <https://developer.stampmitra.in/openapi.json>.

> **Status: pre-1.0 (0.1.0).** The API surface may change in minor releases. Pin your version.

SDK guide: <https://developer.stampmitra.in/sdks/go> | API docs: <https://developer.stampmitra.in/docs>

## Install

```bash
go get github.com/Bani-Global-Industries-LLP/stampmitra-developer-go@v0.1.0
```

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
| `client.Estamp` | `GetSchema`, `ListStates`, `ListInstruments`, `ListArticles`, `CalculateDuty`, `AddParties` |
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
`Attempts`). Classify with `errors.Is` and the sentinels, or the helpers:

| HTTP | Sentinel | Helper |
|---|---|---|
| 400 (and local validation) | `ErrValidation` | `IsValidation` |
| 401 | `ErrAuthentication` | `IsAuthentication` |
| 402 | `ErrPaymentRequired` | `IsPaymentRequired` |
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
capped at 60s) overrides the computed delay; the API itself currently sends none. Retried: 429, 502, 503, 504, connection errors and
timeouts. Not retried: other statuses (400, 401, 402, 403, 404, 409, 500 ...). GET is always retry-safe.

The API honours `Idempotency-Key` on every metered call, so the SDK generates a random UUIDv4 key (from `crypto/rand`; it fails
rather than fall back to a weak generator) for **every POST except webhook replay** and reuses the **same key across automatic retries**.
A retried verification, order or payment intent therefore replays the original result instead of being billed or created twice.
Pass your own with `WithIdempotencyKey` (use one derived from your own order id when your program itself may re-run). A POST
without a key (webhook replay) is never retried unless you give it one.

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

The SDK is free and MIT-licensed. You pay only for API calls, and only for successful production calls; Sandbox is free.

| Plan | Rate |
|---|---|
| **Sandbox** (test keys, `sk_test_...`) | Free. Calls run against sandbox data and never debit your wallet. |
| **Developer Suite** (live keys, `sk_live_...`) | ₹2 per call: PAN, GSTIN, vehicle RC, Voter ID, Driving Licence, Passport, CIN, Udyam, PAN to GSTIN, PAN to Udyam |
| | ₹4 per call: bank account (penny-drop) |
| | ₹5 per call: each e-Stamp call |
| **White-label / CA fulfilment** | Custom, with volume pricing and SLAs. [Contact us](https://developer.stampmitra.in/contact). |

Production calls are deducted from your API credit wallet (top up under Dashboard, Billing). Rates are shown for convenience and may change; the [pricing page](https://developer.stampmitra.in/pricing) is the source of truth.

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
