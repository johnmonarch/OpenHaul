<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 11 Agent Build Plan

## 1. Agent objective

Build OpenHaul Guard OSS as a production-quality Go CLI with local SQLite storage, lazy FMCSA lookup, deterministic risk scoring, Markdown/JSON reports, and a local MCP server.

## 2. Implementation rules for agents

- Do not invent unsupported API endpoints.
- Use official docs listed in `03_DATA_SOURCES_AND_INGESTION.md`.
- Keep schemas stable and shared between CLI and MCP.
- Add tests with every module.
- Do not add telemetry.
- Do not send packet contents to external services.
- Do not use fraud-blacklist language.
- Keep setup easy for non-technical users.

## 3. Suggested task sequence

### Task 1: Project skeleton

Create Go module and repo structure.

Acceptance:

```bash
go test ./...
go run ./cmd/ohg --version
```

### Task 2: Config and setup quick mode

Implement:

```bash
ohg setup --quick --yes
ohg doctor
```

Acceptance:

- Creates OHG home.
- Creates config.
- Creates SQLite DB.
- Runs migrations.

### Task 3: Credential storage

Implement:

```bash
ohg setup fmcsa
ohg setup socrata
```

Acceptance:

- Opens browser unless `--no-browser`.
- Accepts masked input.
- Stores credentials securely.
- Validates or gives helpful error.

### Task 4: FMCSA QCMobile client

Implement endpoints from official docs.

Acceptance:

- Fixture tests pass.
- Live tests are gated by env vars.
- No WebKey in logs.

### Task 5: Carrier lookup orchestration

Implement:

```bash
ohg carrier lookup --dot <number>
ohg carrier lookup --mc <number>
```

Acceptance:

- Resolves MC to USDOT.
- Stores raw payload metadata.
- Stores normalized carrier.
- Stores observation.
- Outputs JSON and Markdown.

### Task 6: Scoring engine

Implement rules in `05_SCORING_RULES.md`.

Acceptance:

- Each rule has tests.
- Flags include evidence.
- No disallowed language.

### Task 7: Diff engine

Implement:

```bash
ohg carrier diff --mc <number> --since 90d
```

Acceptance:

- Material diffs show old and new values.
- Strict mode shows formatting changes.

### Task 8: Watchlist

Implement:

```bash
ohg watch add --mc <number>
ohg watch list
ohg watch sync
```

Acceptance:

- Watchlist persists.
- Sync creates observations.
- Report summarizes changes.

### Task 9: Packet checker MVP

Implement PDF text extraction and field comparisons.

Acceptance:

- Handles a simple text-based PDF fixture.
- Produces mismatch report.
- Fails gracefully if PDF extraction dependency is missing.

### Task 10: MCP server

Implement:

```bash
ohg mcp serve
```

Acceptance:

- Supports `carrier_lookup` and `carrier_diff` at minimum.
- Uses same service layer as CLI.
- Returns schema-compatible JSON.

### Task 11: Packaging

Implement GoReleaser and GitHub Actions.

Acceptance:

- Builds on macOS, Linux, Windows.
- Checksums generated.
- Homebrew formula generated or documented.

## 4. Hidden test flag

Agents may implement a hidden `--fixture` flag to accelerate testing.

Rules:

- Hidden from normal help.
- Only used in tests and local dev.
- Must never be required by production flows.

## 5. Quality bar

Do not consider the project usable until this command sequence works:

```bash
ohg setup --quick --yes
ohg doctor --format json
ohg carrier lookup --mc 123456 --format json
ohg carrier lookup --mc 123456 --format markdown
ohg carrier diff --mc 123456 --since 90d --format json
ohg mcp serve --help
```

## 6. Key implementation caution

FMCSA and DOT source schemas may change. Build source adapters so schema changes are localized to `internal/sources/*` and `internal/normalize/*`.

Do not scatter source field names throughout CLI, scoring, reporting, or MCP code.
