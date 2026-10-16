# Developer commands. CI runs the same checks (see .github/workflows/ci.yml).
SPEC_DIR ?= /home/user/stampmitra-developer-openapi
export STAMPMITRA_SPEC_DIR ?= $(SPEC_DIR)

.PHONY: check test race conformance coverage-spec lint examples readme fmt vuln

check: fmt lint race coverage-spec readme

fmt:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run gofmt -w ."; exit 1; }

lint:
	go vet ./... && go vet -tags conformance ./...
	staticcheck ./... && staticcheck -tags conformance ./...
	golangci-lint run ./... && golangci-lint run --build-tags conformance ./...

test:
	go test ./...

race:
	go test -race -cover -count=1 ./...

# Fails when an operation in spec/openapi.json has no SDK method.
coverage-spec:
	go test -run TestSpecCoverage -count=1 -v .

# Starts the spec repo's mock server (needs Node) on a free port and runs the conformance suite.
conformance:
	CONFORMANCE_REQUIRED=1 go test -tags conformance -race -count=1 ./...

# Compiles every Go snippet in README.md.
readme:
	python3 scripts/check_readme.py

examples:
	./scripts/run-examples.sh

vuln:
	govulncheck ./...
