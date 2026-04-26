<div align="center">

# OpenHaul Guard

**Local-first carrier verification and freight risk review for brokers, carriers, 3PLs, shippers, and compliance teams.**

[![CI](https://github.com/johnmonarch/OpenHaul/actions/workflows/ci.yml/badge.svg)](https://github.com/johnmonarch/OpenHaul/actions/workflows/ci.yml)
[![Security](https://github.com/johnmonarch/OpenHaul/actions/workflows/security.yml/badge.svg)](https://github.com/johnmonarch/OpenHaul/actions/workflows/security.yml)
[![Release](https://github.com/johnmonarch/OpenHaul/actions/workflows/release.yml/badge.svg)](https://github.com/johnmonarch/OpenHaul/actions/workflows/release.yml)
[![Latest Release](https://img.shields.io/github/v/release/johnmonarch/OpenHaul?label=release)](https://github.com/johnmonarch/OpenHaul/releases)
[![License](https://img.shields.io/github/license/johnmonarch/OpenHaul)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/johnmonarch/OpenHaul)](go.mod)
[![Homebrew](https://img.shields.io/badge/homebrew-johnmonarch%2Fopenhaulguard-blue)](https://github.com/johnmonarch/homebrew-openhaulguard)

[Install](INSTALL.md) | [CLI](CLI_REFERENCE.md) | [HTTP API](API.md) | [Watchlist Ops](docs/watchlist-operations.md) | [Architecture](ARCHITECTURE.md) | [Security](SECURITY.md)

</div>

OpenHaul Guard is a free, open source terminal tool for comparing public carrier records with local observations. We built it to make carrier vetting easier for smaller shippers, freight brokers, and operators who need professional-grade carrier research without enterprise software cost or complexity. It packages carrier lookup, evidence collection, risk flags, packet checks, watchlists, and local integrations into a simplified workflow that teams can run for free, and that developers can build carrier intelligence into. OpenHaul Guard is an evidence tool, not a blacklist: reports separate source facts from OpenHaul Guard risk flags and are intended for manual review.

## What It Does

| Area | Capability |
| --- | --- |
| Carrier lookup | FMCSA QCMobile live lookup with a user-provided WebKey, local cache, offline reuse, and JSON bootstrap mirror fallback |
| Reports | Carrier lookup, risk flags, diffs, watchlist reports, and packet checks in table, JSON, or Markdown |
| Watchlists | Add carriers, sync observations, export JSON, and run scheduled local monitoring jobs |
| Packet checks | Extract text/PDF packet fields and compare MC/DOT/name/address/phone/email/insurance/payment details against lookup data |
| Integrations | Local HTTP API for server-side websites and internal tools, plus developer-preview MCP over stdio |
| Storage | Local SQLite, local raw/report/log directories, no hosted service required |

## Quick Start

Install the latest release:

```bash
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/latest/download/install.sh | sh
```

Or install with Homebrew:

```bash
brew tap johnmonarch/openhaulguard
brew install ohg
```

Initialize local config and storage:

```bash
ohg setup --quick --yes
ohg doctor --format json
```

Run a first lookup:

```bash
ohg carrier lookup --mc 123456 --format markdown
```

For guided setup, run `ohg setup`. For script-only local bootstrap, run `ohg init`.

## Try Without Credentials

Run a fixture-backed lookup without live FMCSA credentials:

```bash
ohg carrier lookup --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format markdown
```

Build or import a local bootstrap mirror for no-key lookup fallback:

```bash
ohg mirror build examples/fixtures/socrata/company_census_rows.json
ohg carrier lookup --mc 123456 --format json
```

## Live Lookup

Live FMCSA lookup requires a free FMCSA WebKey:

```bash
ohg setup fmcsa
ohg carrier lookup --mc 123456 --format json
```

## Packet Checks

Extract fields from a text carrier packet:

```bash
ohg packet extract examples/fixtures/packets/basic_carrier_packet.txt \
  --format json
```

Check a text carrier packet against a lookup result:

```bash
ohg packet check examples/fixtures/packets/basic_carrier_packet.txt \
  --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format json
```

## Local HTTP API

Run the local API for server-side website integrations:

```bash
ohg serve --listen 127.0.0.1:8787
```

Then call it from backend code:

```bash
curl -s http://127.0.0.1:8787/v1/carrier/lookup \
  -H "Content-Type: application/json" \
  -d '{"identifier_type":"mc","identifier_value":"123456","max_age":"24h"}'
```

## Development

Build from source:

```bash
go build -o ohg ./cmd/ohg
```

## Documentation

- [Install](INSTALL.md)
- [CLI reference](CLI_REFERENCE.md)
- [Configuration](CONFIGURATION.md)
- [Data sources](DATA_SOURCES.md)
- [HTTP API](API.md)
- [HTTP API service operation](docs/http-api-service.md)
- [MCP](MCP.md)
- [Architecture](ARCHITECTURE.md)
- [Watchlist operations](docs/watchlist-operations.md)
- [Release checklist](docs/release-v0.1.0.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)
- [Code of conduct](CODE_OF_CONDUCT.md)

The original planning/spec bundle remains in `openhaul_guard_spec_docs/`.

## License

Apache License 2.0. See [LICENSE](LICENSE).
