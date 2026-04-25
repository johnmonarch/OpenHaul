# Contributing

OpenHaul Guard is an OSS Go CLI for local-first carrier verification and freight risk review. Contributions should keep the tool practical, auditable, and safe for operators handling carrier records and credentials.

Please read [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) before participating.

## Development Setup

Requirements:

- Go 1.23 or newer
- Git
- Optional: `pdftotext` from poppler for PDF packet checks
- Optional: `actionlint`, `goreleaser`, and `govulncheck` for release/security checks

Build the CLI:

```bash
go build -o ohg ./cmd/ohg
```

Run tests:

```bash
go test ./...
```

Check formatting:

```bash
gofmt -l .
```

Format only files you intentionally changed:

```bash
gofmt -w path/to/file.go
```

## Project Map

- `cmd/ohg`: CLI entrypoint
- `internal/cli`: Cobra commands and user-facing command wiring
- `internal/app`: application orchestration and workflows
- `internal/sources`: FMCSA, Socrata, and local mirror source clients
- `internal/normalize`: carrier source normalization
- `internal/scoring`: risk flags and recommendations
- `internal/store`: SQLite storage and migrations
- `internal/report`: table, Markdown, and JSON writers
- `internal/packet`: carrier packet extraction and checks
- `internal/mcp` and `internal/httpapi`: local integration surfaces
- `schemas`: JSON schema artifacts for public report shapes

See [ARCHITECTURE.md](ARCHITECTURE.md) for the higher-level data flow.

## Local Smoke Test

Use an isolated home directory so development runs do not touch your normal OpenHaul Guard state:

```bash
go build -o ohg ./cmd/ohg

OHG_HOME=/tmp/ohg-dev ./ohg init
OHG_HOME=/tmp/ohg-dev ./ohg doctor --format json
OHG_HOME=/tmp/ohg-dev ./ohg carrier lookup --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format json
OHG_HOME=/tmp/ohg-dev ./ohg packet check examples/fixtures/packets/basic_carrier_packet.txt \
  --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format json
```

Watchlist changes should also exercise JSON output:

```bash
OHG_HOME=/tmp/ohg-dev ./ohg watch add --mc 123456 --label smoke
OHG_HOME=/tmp/ohg-dev ./ohg watch sync \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format json
OHG_HOME=/tmp/ohg-dev ./ohg watch report --since 24h --format json
OHG_HOME=/tmp/ohg-dev ./ohg watch export --format json
```

## Pull Requests

Before opening a pull request:

- Keep changes scoped to the behavior being changed.
- Run `gofmt` on edited Go files.
- Run `go test ./...`.
- Update docs when CLI behavior, config keys, report fields, security posture, or operating procedures change.
- Add or update tests for behavior changes.
- Do not commit local databases, raw payloads, generated release artifacts, API credentials, carrier packet contents, or customer data.

Good pull requests include:

- A short problem statement
- The implementation approach
- User-visible behavior changes
- Validation commands and results
- Known limitations or follow-up work

## Issues

Use the GitHub issue forms for bugs, feature requests, and documentation gaps. Include exact commands, OS/architecture, OpenHaul Guard version, and sanitized output where possible.

Do not include live FMCSA WebKeys, Socrata app tokens, carrier packet contents, private customer information, or other secrets in public issues. Report suspected vulnerabilities through the private process in [SECURITY.md](SECURITY.md).

## CI

GitHub Actions runs:

- `gofmt -l .`
- `go test ./...`
- `go build -trimpath ./cmd/ohg`
- `govulncheck ./...`

The workflows set `GOCACHE` and `GOMODCACHE` under runner-local temp directories.

## Release Work

Release tags use semantic versioning:

```text
v0.1.0
```

Pushing a `v*` tag runs GoReleaser. Release archives are produced for darwin, linux, and windows on amd64 and arm64, with `checksums.txt` and Linux packages.

Maintainers should follow [docs/release-v0.1.0.md](docs/release-v0.1.0.md) for the `v0.1.0` release checklist, including Homebrew tap and `HOMEBREW_TAP_GITHUB_TOKEN` validation.
