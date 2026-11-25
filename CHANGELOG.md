# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/) (pre-1.0: minor versions may break).

## [Unreleased]

## [0.2.1] - 2026-11-25

Patch release: wording and documentation; no API or behaviour changes.

### Changed

- Documentation (README, examples, doc comments) now uses one customer-facing **Service wallet** (the one balance that pays
  verification calls and e-Stamp fees; minimum top-up ₹1,000 plus 18% GST; GST on fees is paid at top-up, and orders debit the
  fees before GST) alongside the **Stamp duty wallet**. `ServiceFeeWallet` and the `WalletServiceFee` / `WalletStampDuty` values
  keep their names (kept for compatibility).
- Documentation: an order can be placed without `PANCardImage`, but a PAN card is mandatory to complete the stamp. SDK
  validation and the `PAN_CARD_RECOMMENDED` advisory are unchanged.
- Documentation: e-Stamp pricing per paper (₹99 procurement + ₹5 API charge + convenience fee, by default 5% of the paper's
  duty above ₹1,000; duty multiplied by quantity). Sandbox quotes use the same default rates as live. New accounts get no
  signup credit.
- README: the install line is `@v0.2.1`; the tag is created at release.

## [0.2.0] - 2026-11-24

Tracks API contract 1.2.0 (62 operations, up from 31). Pre-1.0: this release contains breaking changes, listed under *Changed*.

### Added
- e-Stamp orders: `Estamp.Orders` (`Validate`, `Create`, `List`, `ListAll`, `Get`, `Cancel`, `Export`, `ListStamps`, `UploadDocument`) and
  `Estamp.Stamps` (`Get`, `Cancel`, `Replace`, `SetPANCard`, `GetCertificate`).
- e-Stamp reference data: `Estamp.GetStatesAvailability`, `GetState`, `GetStateRequirements` (with its `ExampleBody`), `ListDistricts`, `ListSROs`, `GetWallet`.
- `StampDutyWallet` and `ServiceFeeWallet` (`GetBalance`, `Topup`, `ListTransactions`, `ListInvoices`, `GetInvoice` and `ListAll*` iterators) and
  `Account` (`GetBalance`, `GetUsage`).
- `Party` (a bare name string or an `EstampParty` object), `EstampPropertyArea`, and the new request and response types.
- `APIError.FieldErrors` (`error.details.errors[]`), `APIError.Wallet` (`WalletStampDuty` / `WalletServiceFee` on a 402) and `APIError.Details`.
- Generic `Iterator[T]` for cursor-paginated lists.
- `Estamp.Orders.Create` sends an auto-generated `Idempotency-Key` when you pass none (the API requires one); `UploadDocument` sends `multipart/form-data` (uploaded content and file names are never logged).
- Retry rules from the 1.2.0 contract: a 409 `IDEMPOTENCY_IN_PROGRESS` on a keyed request is retried after `Retry-After`; `Estamp.Orders.Validate` (a dry run) is retried without a key.
- Examples: `examples/estamp_order`.

### Changed
- **Breaking:** `Estamp.Orders.Cancel`, `Estamp.Stamps.Cancel`, `Estamp.Stamps.Replace`, `Estamp.Stamps.SetPANCard`, both wallet `Topup` methods and
  `Webhooks.Events.Replay` are never retried automatically and send no `Idempotency-Key`. `Webhooks.Events.Replay` used to be retried when you passed `WithIdempotencyKey`.
- HTTP 422 now matches `ErrValidation` (it matched nothing before).
- **Breaking:** verification result models follow contract 1.2.0 and gain many fields (including `VerificationID`); types changed where the API changed them:
  `VoterIDVerification.Age` is a `string`, `CINVerification.RegistrationNumber` is an `int64`, `PANToUdyam.UdyamNumber` is a `*string`;
  `GSTINVerification.GSTIN` now holds the lower-case `gstin` field and the legacy upper-case `GSTIN` field is `GSTINLegacy`.
- Debug logging masks the new e-Stamp personal-data fields (parties, PAN card link, payer phone).
- Spec snapshot, shared retry scenarios and docs refreshed to contract 1.2.0; the conformance suite now covers all 62 operations and all 411 golden fixtures.

## [0.1.0] - 2026-10-16

### Added
- Initial release: `Verify` (PAN, GSTIN, RC, Voter ID, driving licence, passport, CIN, Udyam, PAN-to-GSTIN, PAN-to-Udyam, bank account), `Services`, `Estamp`, `Orders`, `Payments.Intents`, `Usage`, `Logs` (with the `LogsIterator`), `Webhooks.Events`.
- `VerifySignature` / `ConstructEvent` / `ConstructEventFromHeaders` (HMAC-SHA256, constant-time, 300s timestamp tolerance) and typed values for the eight `order.*` events.
- Typed response models with `json` tags; unknown fields are preserved in `Extra` and survive re-encoding. `GenericResponse` for endpoints without a fixed shape.
- Typed errors (`*APIError`, `*ValidationError`, `*ConnectionError`, ...) with `errors.Is` sentinels and `Is*` helpers.
- Safe retries with full-jitter backoff and `Retry-After`; an `Idempotency-Key` (UUIDv4 from `crypto/rand`) on every POST except webhook replay, reused across retries.
- Client-side input validators; `slog`-based debug logging with redaction (off by default).
- Per-call options: timeout, max retries, idempotency key, headers, raw response capture.
- Bundled OpenAPI snapshot, a spec-coverage test, a conformance suite against the spec repo's mock server, runnable examples.
- `baseURL` must be https (loopback http allowed) with no credentials; redirects are never followed; the API key and webhook secret are redacted from `fmt` output, errors and logs.

[Unreleased]: https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/releases/tag/v0.1.0
