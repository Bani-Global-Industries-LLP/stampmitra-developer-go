// Package stampmitra is the official Go client for the StampMitra Developer API: Indian identity
// verification (PAN, GSTIN, vehicle RC, Voter ID, driving licence, passport, CIN, Udyam, bank
// account) and e-Stamp Paper, plus orders, payment intents, usage, request logs and webhook events.
//
// The SDK is pre-1.0 (0.2.1): the API surface may change in minor releases, so pin your version.
// It needs Go 1.21 or newer and uses only the standard library.
//
// # Quickstart
//
//	client, err := stampmitra.NewClient("") // reads STAMPMITRA_API_KEY
//	if err != nil {
//		log.Fatal(err)
//	}
//	pan, err := client.Verify.PAN(ctx, &stampmitra.PANVerifyParams{PAN: "ABCDE1234F", Name: "Rahul Kumar"})
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(pan.Valid, pan.ReferenceID)
//
// Keys look like sk_test_... (free sandbox) or sk_live_... (metered); the prefix selects the
// environment (Client.Environment). Keys are for server-side use only: never ship an sk_ key in a
// mobile app, a browser bundle or a public repository.
//
// # Resources
//
// A Client exposes one field per API area: Verify, Services, Estamp (with Estamp.Orders and
// Estamp.Stamps), StampDutyWallet, ServiceFeeWallet, Account, Orders, Payments.Intents, Usage,
// Logs and Webhooks.Events. Every method takes a context.Context first and optional
// RequestOption values last (WithRequestTimeout, WithRequestMaxRetries, WithIdempotencyKey,
// WithRequestHeader, WithRawResponse).
//
// e-Stamp orders: validate with Estamp.Orders.Validate, create with Estamp.Orders.Create (an
// Idempotency-Key is sent automatically), upload the payer's PAN card with UploadDocument, then read
// the stamps and their certificates. A party is a bare name or an object: see Party.
//
// Orders are addressed by YOUR externalOrderId: Orders.Create returns it, and Orders.Get,
// GetStatus, GetPayment, ListDocuments, GetDocumentDownload and Estamp.AddParties take it.
//
// # Models
//
// Responses are structs with json tags. Every response struct has an Extra map holding fields the
// SDK version does not know, so new API fields are never lost and never break decoding. Endpoints
// whose shape the API does not fix (e-Stamp reference data, duty, parties, webhook event lists)
// return *GenericResponse, which keeps the whole body.
//
// # Errors
//
// Non-2xx responses are returned as *APIError (Status, Code, Message, RequestID, Field, RetryAfter, FieldErrors, Wallet).
// Classify errors with errors.Is and the Err* sentinels, or the Is* helpers:
//
//	var apiErr *stampmitra.APIError
//	switch {
//	case stampmitra.IsRateLimit(err):
//		// back off
//	case errors.As(err, &apiErr):
//		log.Printf("request %s failed: %s", apiErr.RequestID, apiErr.Message)
//	}
//
// Input that fails client-side validation is rejected before any network call (and so before any
// billing) with a *ValidationError, which matches ErrValidation. Network failures are
// *ConnectionError (ErrConnection, and ErrTimeout for timeouts). A cancelled context returns the
// context's own error.
//
// # Retries and idempotency
//
// By default a call is attempted up to three times (WithMaxRetries(2)) with exponential backoff
// and full jitter (500ms base, 8s cap). Retry-After is honoured, capped at 60s. 429, 502, 503, 504,
// connection errors and timeouts are retried, as is a 409 IDEMPOTENCY_IN_PROGRESS on a keyed
// request; other statuses are not. GET is always retry-safe.
// The API honours Idempotency-Key on metered POSTs, order creation and document upload, so the SDK
// sends a random UUIDv4 key with each of them (or the key you pass with WithIdempotencyKey) and
// re-uses the SAME key on every automatic retry, so a retried verification or order is never billed
// twice. Order and stamp cancel, replace, PAN-card link, both wallet top-ups and webhook replay are
// never retried and send no key; Estamp.Orders.Validate is a dry run and is retried like a GET.
//
// # Pagination
//
// LogsService.ListAll returns a LogsIterator that follows nextCursor lazily:
//
//	it := client.Logs.ListAll(&stampmitra.LogsListParams{Limit: 100})
//	for it.Next(ctx) {
//		fmt.Println(it.Log().ID)
//	}
//	if err := it.Err(); err != nil {
//		log.Fatal(err)
//	}
//
// # Webhooks
//
// Verification and decoding of webhook deliveries live in this package (VerifySignature,
// ConstructEvent, ConstructEventFromHeaders) so a receiver needs only one import. Deliveries are
// signed with hex HMAC-SHA256(secret, timestamp + "." + rawBody); pass the body exactly as received.
// The eight order.* events decode to typed values (*OrderCreatedEvent, ...); anything else becomes
// *UnknownEvent.
//
// # Security defaults
//
// The base URL must be https (http only for localhost, 127.0.0.1 and ::1) and must not contain
// credentials. Redirects are never followed, so credentials cannot reach another host. The API key
// and webhook secret never appear in String/Format output, error messages or logs. Debug logging
// (WithLogger) is off by default and, when enabled, redacts keys and masks identifiers and
// personal data.
package stampmitra
