# OpenHaul Guard

OpenHaul Guard is a local-first carrier verification and freight risk review CLI. It helps brokers, carriers, 3PLs, shippers, and compliance teams compare public carrier records with local observations.

OpenHaul Guard is an evidence tool, not a blacklist. Reports separate source facts from OpenHaul Guard risk flags and are intended for manual review.

## Current Status

This repository contains the first Go CLI implementation:

- Local setup, config, SQLite storage, and doctor checks
- FMCSA QCMobile live lookup with a user-provided WebKey
- Local cache and offline lookup after a carrier has been observed
- Local JSON bootstrap mirror import and no-key mirror lookup fallback
- Carrier lookup reports in table, JSON, or Markdown
- Local carrier diffs across stored observations
- Watchlist add, remove, list, sync, and reports
- Text/PDF carrier packet extraction and checks against lookup results
- Developer-preview MCP JSON-RPC server over stdio

## Quick Start

Install from the latest release:

```bash
curl -fsSL https://github.com/johnmonarch/OpenCarrier/releases/latest/download/install.sh | sh
```

Or install with Homebrew:

```bash
brew tap johnmonarch/openhaulguard
brew install ohg
```

Initialize local config and storage:

```bash
ohg setup
ohg doctor
```

`ohg setup` is guided and resumable. For a fast local bootstrap in scripts, run `ohg init`.

Build from source for development:

```bash
go build -o ohg ./cmd/ohg
```

Run a fixture-backed lookup without live credentials:

```bash
ohg carrier lookup --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format markdown
```

If you built locally and did not install the binary, use `./ohg`:

```bash
./ohg carrier lookup --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format markdown
```

Import a local bootstrap mirror for no-key lookup fallback:

```bash
ohg mirror import examples/fixtures/mirror/carriers.json
ohg carrier lookup --mc 123456 --format json
```

Live FMCSA lookup requires a free FMCSA WebKey:

```bash
./ohg setup fmcsa
./ohg carrier lookup --mc 123456 --format json
```

Extract fields from a text carrier packet:

```bash
./ohg packet extract examples/fixtures/packets/basic_carrier_packet.txt \
  --format json
```

Check a text carrier packet against a lookup result:

```bash
./ohg packet check examples/fixtures/packets/basic_carrier_packet.txt \
  --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format json
```

## Documentation

- [Install](INSTALL.md)
- [CLI reference](CLI_REFERENCE.md)
- [Configuration](CONFIGURATION.md)
- [Data sources](DATA_SOURCES.md)
- [MCP](MCP.md)
- [Contributing](CONTRIBUTING.md)
- [Security](SECURITY.md)

The original planning/spec bundle remains in `openhaul_guard_spec_docs/`.

## License

Apache License 2.0. See [LICENSE](LICENSE).
