# Security Policy

Please report vulnerabilities privately to **security@stampmitra.in**
(see https://developer.stampmitra.in/.well-known/security.txt). Do not open
public issues for security problems. We aim to acknowledge reports within
3 business days.

Never include live API keys (`sk_live_...`), webhook secrets or personal data in a report.

## Supported versions

Pre-1.0: only the latest 0.x release receives fixes.

## Using the SDK safely

- Server-side only. Never embed `sk_` keys in mobile apps, browsers or public repos.
- Load keys from the environment or a secrets manager.
- Verify webhook signatures on the raw body (`stampmitra.ConstructEventFromHeaders`) and keep the default 300s timestamp tolerance.
- The debug logger (`WithLogger`) redacts credentials and masks identifiers and names, but treat logs as sensitive anyway.
- Run `govulncheck ./...` in your own build: the SDK has no third-party dependencies, so findings come from the Go standard library; keep your Go toolchain patched.
