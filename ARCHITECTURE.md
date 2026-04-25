# Architecture

OpenHaul Guard is a local-first Go CLI. The core workflow is: accept a carrier identifier, fetch or reuse source data, normalize it into stable local models, score risk flags, persist observations, and render reports for human review.

```mermaid
flowchart LR
  user["Operator or integration"] --> cli["ohg CLI (Cobra)"]
  cli --> app["Application workflows"]
  app --> config["Config and credentials"]
  app --> sources["Source clients"]
  app --> store["SQLite store"]
  app --> scoring["Risk scoring"]
  app --> reports["Report writers"]
  sources --> fmcsa["FMCSA QCMobile"]
  sources --> socrata["DOT DataHub / Socrata"]
  sources --> mirror["Local bootstrap mirror"]
  config --> keychain["OS keychain or 0600 fallback"]
  store --> disk["OHG_HOME local files"]
  reports --> output["Table, JSON, Markdown"]
  app --> mcp["MCP stdio server"]
  app --> http["Local HTTP API"]
```

## Main Components

- `cmd/ohg`: executable entrypoint.
- `internal/cli`: Cobra command definitions, global flags, and user-facing command wiring.
- `internal/app`: orchestration layer for setup, lookup, diff, watchlist, mirror, packet, MCP, and HTTP workflows.
- `internal/config`: default config, TOML loading, environment overrides, and local directory layout.
- `internal/credentials`: secret storage abstraction using the OS keychain when available.
- `internal/sources`: network and local source clients.
- `internal/normalize`: conversion from source-specific payloads into domain models.
- `internal/scoring`: deterministic risk flags, evidence, and recommendations.
- `internal/store`: SQLite persistence and migrations.
- `internal/report`: table, Markdown, and JSON report rendering.
- `internal/packet`: text and PDF carrier packet extraction and comparison.
- `internal/mcp`: local MCP JSON-RPC server over stdio.
- `internal/httpapi`: local HTTP API for trusted backend integrations.

## Lookup Data Flow

```mermaid
sequenceDiagram
  participant U as Operator
  participant CLI as "ohg carrier lookup"
  participant App as "internal/app"
  participant Store as "SQLite store"
  participant Src as "FMCSA or mirror"
  participant Score as "Risk scoring"
  participant Report as "Report writer"

  U->>CLI: "lookup --mc 123456 --format json"
  CLI->>App: "LookupRequest"
  App->>Store: "read fresh cached observation"
  alt "cache hit and max-age valid"
    Store-->>App: "stored observation"
  else "cache miss or force refresh"
    App->>Src: "fetch carrier source data"
    Src-->>App: "raw source payload"
    App->>Store: "persist raw metadata and normalized observation"
  end
  App->>Score: "assess normalized carrier"
  Score-->>App: "risk assessment"
  App->>Store: "persist assessment and flags"
  App->>Report: "render requested format"
  Report-->>U: "table, JSON, or Markdown"
```

## Local State

By default, local state lives under `~/.openhaulguard`. Operators can override this with `OHG_HOME` or `--home`.

Typical layout:

```text
~/.openhaulguard/
  config.toml
  ohg.db
  raw/
  reports/
  logs/
  mirror/carriers.json
```

The SQLite database stores normalized carrier observations, risk assessments, setup state, packet checks, and watchlist entries. Raw source metadata and reports stay local unless an operator exports or shares them.

## Integration Surfaces

- CLI commands are the primary interface and should stay scriptable with stable JSON output.
- The HTTP API is intended for same-host backend integrations. It binds to loopback by default and should use a token before non-loopback binding.
- The MCP server is a developer-preview stdio integration surface for local assistants and tools.
- Watchlist operations are intentionally pull-based. Use scheduled `watch sync`, `watch report --format json`, and `watch export --format json` jobs instead of relying on hosted webhooks.

## Design Constraints

- Local-first: no hosted service is required for normal CLI operation.
- Human review: OpenHaul Guard provides evidence and flags, not tendering decisions or carrier blacklists.
- Secret minimization: credentials should not appear in reports, logs, raw metadata, or public issues.
- Deterministic reports: JSON output should remain suitable for automation and regression tests.
- Conservative dependencies: prefer standard library and small, well-maintained packages for a CLI footprint.
