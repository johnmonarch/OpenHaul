# OpenHaul Guard OSS

OpenHaul Guard is a local-first carrier verification and freight risk review CLI for trucking companies, freight brokers, 3PLs, shippers, and compliance workflows.

It is an evidence engine, not a blacklist. The tool separates public-record facts, normalized local observations, and OpenHaul Guard risk flags intended for manual review.

## Current Build

This repository now contains the first Go implementation pass:

```bash
ohg setup --quick --yes
ohg doctor --format json
ohg carrier lookup --mc 123456 --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json --format markdown
ohg carrier diff --mc 123456 --since 90d --format json
ohg watch add --mc 123456
ohg watch list --format json
ohg mcp serve --help
```

Live FMCSA lookup requires a user-provided FMCSA WebKey:

```bash
ohg setup fmcsa
```

Quick setup initializes local config and SQLite storage without government API keys. Until a redistributable bootstrap mirror is published, arbitrary fresh carrier lookup requires FMCSA credentials or a test fixture.

## Implementation Notes

- Primary language: Go
- CLI framework: Cobra
- Local database: SQLite via `modernc.org/sqlite`
- Credential storage: OS keychain via `go-keyring`, with `0600` local fallback
- Default telemetry: off
- License: Apache-2.0

The full planning/spec bundle lives in `openhaul_guard_spec_docs/`.
