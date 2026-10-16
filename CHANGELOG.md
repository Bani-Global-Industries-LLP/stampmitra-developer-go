# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres
to [Semantic Versioning](https://semver.org/) (pre-1.0: minor versions may break).

## [Unreleased]

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

[Unreleased]: https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-go/releases/tag/v0.1.0
