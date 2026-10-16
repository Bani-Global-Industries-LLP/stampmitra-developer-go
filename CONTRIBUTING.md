# Contributing

Thanks for helping improve the StampMitra Go SDK.

## Setup

Go 1.21 or newer (the module declares `go 1.21`; do not use APIs added later, CI tests 1.21 to 1.24).

```bash
make check             # gofmt, go vet, staticcheck, golangci-lint, go test -race -cover, spec coverage
make coverage-spec     # every operation in spec/openapi.json must have an SDK method
```

Tools: `go install honnef.co/go/tools/cmd/staticcheck@latest`, golangci-lint v2 (https://golangci-lint.run), `govulncheck` (`go install golang.org/x/vuln/cmd/govulncheck@latest`).

### Conformance suite

Runs all 31 operations, error mapping, retries, idempotency, timeouts, pagination and the shared vectors against the spec
repo's mock server. It starts the server itself (`os/exec`, free port), so you need Node and a checkout of the spec repo:

```bash
git clone https://github.com/Bani-Global-Industries-LLP/stampmitra-developer-openapi ../stampmitra-developer-openapi
STAMPMITRA_SPEC_DIR=../stampmitra-developer-openapi make conformance
```

`STAMPMITRA_SPEC_DIR` defaults to `/home/user/stampmitra-developer-openapi`. Without a checkout the conformance tests are skipped
locally; CI sets `CONFORMANCE_REQUIRED=1` so they fail instead. CI checks out the spec repo's default branch, so that repository
needs a `main` branch to exist. `make examples` runs every program in `examples/` against the same mock.

`testdata/{validators,webhook-signatures,retry-scenarios}.json` and `testdata/webhooks/` are copies of the shared fixtures in the
spec repo; the conformance suite fails when they drift. `spec/openapi.json` is a snapshot of the public spec.

### Generated code

`models.go` and `params.go` are generated from the spec: edit `scripts/gen_models.py`, then
`python3 scripts/gen_models.py && gofmt -w models.go params.go`. When the API changes, refresh `spec/openapi.json`, regenerate, and add the
new SDK methods; `TestSpecCoverage` fails until you do.

## Guidelines

- Standard library only at runtime. Examples live in a separate module (`examples/go.mod`).
- Add tests for every change; tests use `httptest`, never the real API. Never log or commit real keys or personal data.
- Error messages must never contain the API key, a webhook secret or user-supplied identifiers.
- Update `CHANGELOG.md` under *Unreleased*.
- By contributing you agree your work is licensed under the MIT license.

Security issues: see [SECURITY.md](SECURITY.md), do not open a public issue.
