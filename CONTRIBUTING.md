# Contributing

## Development Setup

Requirements:

- Go 1.23 or newer
- Git

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

## Local Smoke Test

Use an isolated home directory:

```bash
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

## Pull Requests

Before opening a pull request:

- Keep changes scoped to the behavior being changed
- Run `gofmt` on edited Go files
- Run `go test ./...`
- Update docs when CLI behavior, config keys, or report fields change
- Do not commit local databases, raw payloads, generated release artifacts, or credentials

## CI

GitHub Actions runs:

- `gofmt -l .`
- `go test ./...`
- `go build -trimpath ./cmd/ohg`
- `govulncheck ./...`

The workflows set `GOCACHE` and `GOMODCACHE` under the runner temp directory.

## Releases

Release tags should use semantic versioning:

```text
v0.1.0
```

Pushing a `v*` tag runs GoReleaser. Release archives are produced for darwin, linux, and windows on amd64 and arm64, with `checksums.txt`.
