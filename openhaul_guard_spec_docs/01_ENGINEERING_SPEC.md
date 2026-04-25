<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 01 Engineering Specification

## 1. Overview

This document defines how to build OpenHaul Guard OSS.

Implementation language: Go
Minimum Go version: 1.23 or current stable at implementation time
Primary datastore: SQLite
Optional analytics datastore: DuckDB, post-MVP
Primary interface: CLI
Secondary interface: local MCP server
Optional local HTTP API: post-MVP unless needed by MCP implementation

## 2. Architecture

OpenHaul Guard is composed of these modules:

```text
cmd/ohg                  CLI entrypoint
internal/app             application orchestration
internal/config          config loading and validation
internal/setup           guided onboarding wizard
internal/credentials     OS keychain and fallback credential storage
internal/store           SQLite store, migrations, queries
internal/sources/fmcsa   FMCSA QCMobile client
internal/sources/socrata Socrata/DOT DataHub client
internal/sources/mirror  public bootstrap mirror client
internal/ingest          lookup and sync orchestration
internal/normalize       source-specific mapping into canonical schema
internal/scoring         deterministic rules engine
internal/diff            snapshot comparison engine
internal/report          table, JSON, Markdown, HTML/PDF outputs
internal/packet          packet text extraction and comparison
internal/mcp             MCP server and tool handlers
internal/logging         structured logging
internal/version         build info
schemas                  JSON Schemas and OpenAPI-like examples
docs                     user and developer documentation
skills                   SKILL.md copied from docs/08_SKILL.md
```

## 3. MVP modules

### ENG-001 CLI entrypoint

The binary must be named `ohg`.

Required command groups:

```text
ohg setup
ohg doctor
ohg carrier lookup
ohg carrier diff
ohg watch add
ohg watch list
ohg watch sync
ohg packet check
ohg mcp serve
ohg config get
ohg config set
```

Acceptance criteria:

- All commands have `--help`.
- All commands return meaningful nonzero exit codes on failure.
- All output commands support `--format table`, `--format json`, and `--format markdown` unless explicitly not applicable.
- JSON output must be valid JSON with stable field names.

### ENG-002 Local config

Default paths:

```text
Config: ~/.openhaulguard/config.toml
Database: ~/.openhaulguard/ohg.db
Raw payloads: ~/.openhaulguard/raw/
Reports: ~/.openhaulguard/reports/
Setup state: ~/.openhaulguard/setup-state.json
Logs: ~/.openhaulguard/logs/ohg.log
```

Environment variables:

```text
OHG_HOME
OHG_CONFIG
OHG_DB_PATH
OHG_FMCSA_WEBKEY
OHG_SOCRATA_APP_TOKEN
OHG_MODE
OHG_LOG_LEVEL
```

Precedence:

1. CLI flag
2. Environment variable
3. OS keychain credential
4. Config file
5. Default value

### ENG-003 Credential storage

Use OS-native credential storage when possible.

Preferred library: `github.com/zalando/go-keyring` or equivalent.

Credential keys:

```text
service=openhaulguard, user=fmcsa_webkey
service=openhaulguard, user=socrata_app_token
service=openhaulguard, user=hosted_api_key
```

Fallback:

If keychain access fails, store in `~/.openhaulguard/secrets.toml` with file mode `0600` and warn the user.

Never print secrets in logs.

### ENG-004 SQLite store

Use migrations. Preferred library: `pressly/goose`, `golang-migrate`, or a minimal internal migration runner.

The store must:

- Initialize automatically in `ohg setup`.
- Run migrations on startup unless `--no-migrate` is passed.
- Use WAL mode.
- Store raw source payload metadata and normalized snapshots.
- Allow deterministic diffing between observations.

### ENG-005 Ingestion orchestrator

The ingestion orchestrator accepts a lookup request:

```go
type LookupRequest struct {
    IdentifierType string // mc, mx, ff, dot, name
    IdentifierValue string
    ForceRefresh bool
    Offline bool
    MaxAge time.Duration
}
```

It returns:

```go
type LookupResult struct {
    Carrier CarrierProfile
    Sources []SourceFetchResult
    Observation CarrierObservation
    RiskAssessment RiskAssessment
    Freshness FreshnessSummary
    Warnings []UserWarning
}
```

Required behavior:

1. Normalize the input identifier.
2. Resolve to USDOT when possible.
3. Check cache and freshness rules.
4. Fetch from live source if needed and credentials are available.
5. Fall back to local mirror/bootstrap index if live source is unavailable.
6. Persist raw payloads and normalized records.
7. Run scoring.
8. Return reportable result.

### ENG-006 Normalization

All source responses must map into canonical models. Do not expose raw FMCSA or Socrata field names as primary public output fields.

Maintain source field lineage:

```json
{
  "field": "carrier.legal_name",
  "value": "EXAMPLE TRUCKING LLC",
  "source": "fmcsa_qcmobile_carrier",
  "source_field": "content.carrier.legalName",
  "observed_at": "2026-04-25T14:30:00Z"
}
```

### ENG-007 Risk scoring

Use deterministic, explainable rules. No machine-learning model in MVP.

Each risk flag must include:

- Code
- Severity
- Category
- Evidence fields
- Explanation
- Recommended manual review action
- Confidence level

### ENG-008 Diff engine

Diffs compare normalized observations.

Required command:

```bash
ohg carrier diff --mc 123456 --since 90d
```

Diff output must include:

- Field path
- Previous value
- Current value
- Previous observed timestamp
- Current observed timestamp
- Source names

Ignore insignificant formatting differences by default:

- Case-only differences
- Common punctuation differences in names
- Whitespace differences
- Phone formatting differences

Expose strict mode:

```bash
ohg carrier diff --mc 123456 --since 90d --strict
```

### ENG-009 Packet checker

MVP packet checker should support PDF and image-free text extraction.

Allowed MVP approach:

- Use `pdftotext` if present.
- Use Go PDF text extraction library if stable.
- If no extractor is available, show clear install instructions.
- OCR is post-MVP.

Packet checker extracts:

- Legal name
- DBA
- USDOT
- MC/MX/FF
- Address
- Phone
- Email
- Insurance company
- Policy number
- Certificate holder if present

Packet checker compares extracted fields to normalized source facts using exact and fuzzy comparisons.

### ENG-010 MCP server

The MCP server is local-first and should call the same application service layer as CLI commands.

Required command:

```bash
ohg mcp serve
```

The MCP server must not use different schemas from the CLI. It should return the same JSON-compatible objects as `ohg carrier lookup --format json`.

### ENG-011 Logging

Default log level: warn
Debug log enabled by flag or config.

Log destinations:

- Human-readable errors to stderr
- Structured logs to file

Do not log secrets or raw packet contents by default.

### ENG-012 Error handling

Use typed errors:

```go
type OHGError struct {
    Code string
    Message string
    Cause error
    UserAction string
    Retryable bool
}
```

Examples:

```text
OHG_AUTH_FMCSA_MISSING
OHG_AUTH_FMCSA_INVALID
OHG_SOURCE_RATE_LIMITED
OHG_SOURCE_NOT_FOUND
OHG_PACKET_PARSE_FAILED
OHG_DB_MIGRATION_FAILED
OHG_OFFLINE_CACHE_MISS
```

Each user-facing error must include a suggested next command when possible.

## 4. Repository structure

```text
openhaulguard/
  cmd/
    ohg/
      main.go
  internal/
    app/
    config/
    credentials/
    setup/
    store/
      migrations/
    sources/
      fmcsa/
      socrata/
      mirror/
    ingest/
    normalize/
    scoring/
    diff/
    report/
    packet/
    mcp/
    logging/
    version/
  schemas/
    lookup-result.schema.json
    risk-assessment.schema.json
    carrier-profile.schema.json
  docs/
  skills/
    SKILL.md
  examples/
    fixtures/
      fmcsa_qcmobile/
      socrata/
      packets/
  scripts/
  .github/
    workflows/
  go.mod
  go.sum
  README.md
  LICENSE
```

## 5. Suggested dependencies

CLI:

- `github.com/spf13/cobra`
- `github.com/spf13/viper` or minimal config loader

SQLite:

- `modernc.org/sqlite` for CGO-free builds, or `github.com/mattn/go-sqlite3` if CGO builds are acceptable

TUI/prompts:

- `github.com/charmbracelet/huh`
- `github.com/charmbracelet/lipgloss` only if it does not complicate plain output

Tables:

- `github.com/jedib0t/go-pretty/v6/table` or equivalent

Keychain:

- `github.com/zalando/go-keyring`

PDF extraction:

- Prefer external `pdftotext` MVP fallback
- Evaluate Go-native libraries separately

MCP:

- Use the official or most widely adopted Go MCP SDK at implementation time
- If SDK quality is poor, implement JSON-RPC-compatible MCP transport with stdio first

## 6. Build stages

### Stage 1: CLI skeleton and config

Deliver:

- `ohg --version`
- `ohg setup --quick`
- `ohg doctor`
- Config file creation
- DB initialization

### Stage 2: FMCSA live lookup

Deliver:

- FMCSA WebKey setup
- `ohg carrier lookup --dot`
- `ohg carrier lookup --mc`
- Raw payload storage
- Normalized carrier profile

### Stage 3: Observations and diffs

Deliver:

- `carrier_observations`
- Snapshot hashing
- `ohg carrier diff`

### Stage 4: Scoring rules

Deliver:

- Deterministic rules engine
- Risk flags in table, JSON, Markdown

### Stage 5: Watchlist

Deliver:

- `ohg watch add`
- `ohg watch list`
- `ohg watch sync`

### Stage 6: Packet checker

Deliver:

- PDF text extraction
- Field extraction
- Packet comparisons

### Stage 7: MCP server and SKILL.md

Deliver:

- `ohg mcp serve`
- MCP tools defined in `07_MCP_SPEC.md`
- `skills/SKILL.md`

### Stage 8: Full sync and Socrata/DataHub integration

Deliver:

- Socrata token setup
- Company Census index sync
- Selected operating authority history sync

## 7. Definition of done for MVP

MVP is complete when:

- A fresh install can run setup and lookup a carrier by MC.
- The same lookup stores raw and normalized data locally.
- The second lookup creates a second observation and can show a diff.
- Risk flags include explanations and evidence.
- The MCP server can call carrier lookup and return JSON.
- Docs include setup instructions and source/API requirements.
- Tests pass on macOS, Linux, and Windows.
