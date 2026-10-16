# Releasing

Go modules are published by pushing a version tag: there is no registry account, token or secret. The
[module proxy](https://proxy.golang.org) fetches the tag from GitHub and [pkg.go.dev](https://pkg.go.dev) indexes it from the proxy.

## The module path is case-sensitive

The module path is `github.com/Bani-Global-Industries-LLP/stampmitra-developer-go`, with the capital letters of the GitHub
organisation. GitHub URLs are case-insensitive but Go module paths are not:

- consumers must import it with exactly this casing (`go get github.com/Bani-Global-Industries-LLP/stampmitra-developer-go@v0.1.0`);
  a lower-cased path fails with `module declares its path as: ... but was required as: ...`;
- the proxy stores such paths with each capital written as `!` plus the lower-case letter, so the 0.1.0 `.info` URL is
  `https://proxy.golang.org/github.com/!bani-!global-!industries-!l!l!p/stampmitra-developer-go/@v/v0.1.0.info`;
- never rename the GitHub organisation or repository without publishing a new module path; do not change the `module` line in `go.mod`.

## One-time setup

1. Optional: set the repository variable `RELEASE_ENABLED=true` (Settings > Secrets and variables > Actions > Variables).
   Until it is set the `Release` workflow still builds and tests the tag, but does not create the GitHub release or poke the proxy.
2. Optional: protect the `v*` tag pattern (Settings > Rules > Rulesets), since a Go tag is immutable once the proxy has seen it.
3. The repository must be public for pkg.go.dev to show it (the licence, MIT, is in `LICENSE`).

## Per release

1. Move the *Unreleased* notes into a new section in `CHANGELOG.md`.
2. Set `Version` in `version.go` (the workflow fails if it does not match the tag).
3. `make check && make conformance`.
4. `git tag v0.1.0 && git push origin v0.1.0`.
5. The `Release` workflow tests the tag, creates the GitHub release (when `RELEASE_ENABLED=true`) and asks the proxy to fetch it.

## pkg.go.dev indexing

pkg.go.dev picks a version up automatically the first time anyone requests it through the proxy, and the workflow does that for
you. To trigger it by hand (from any machine, with the exact casing):

```bash
GOPROXY=https://proxy.golang.org GO111MODULE=on go list -m github.com/Bani-Global-Industries-LLP/stampmitra-developer-go@v0.1.0
```

Then open `https://pkg.go.dev/github.com/Bani-Global-Industries-LLP/stampmitra-developer-go@v0.1.0` (or click "Request" on that page).
Documentation appears within a few minutes. Retracting a bad release is done with a `retract` directive in `go.mod` in a later
version, never by moving or deleting the tag.

## Versioning

Pre-1.0: minor versions may break. From v2 on the module path must end in `/v2` (a new major version needs a new import path).
`examples/` is a separate module (`examples/go.mod`) and is not part of the published module.
