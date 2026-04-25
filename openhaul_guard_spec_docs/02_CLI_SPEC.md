<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 02 CLI Specification

## 1. Global command behavior

Binary: `ohg`

Global flags:

```text
--config <path>          Path to config file
--home <path>            Override OHG home directory
--format <format>        table, json, markdown, html
--verbose                Enable verbose user output
--debug                  Enable debug logs
--offline                Do not call network sources
--no-color               Disable terminal colors
--yes                    Accept safe defaults in prompts
--version                Print version
--help                   Print help
```

Output rules:

- Human output goes to stdout.
- Errors go to stderr.
- Machine output must never include progress spinners.
- `--format json` must output only JSON.
- Secrets must never be printed.

Exit codes:

```text
0 success
1 generic error
2 invalid command or arguments
3 setup incomplete
4 source authentication failed
5 source unavailable
6 carrier not found
7 local database error
8 packet parse error
9 risk policy violation in command usage
10 offline cache miss
```

## 2. `ohg setup`

### Command

```bash
ohg setup [--quick] [--keys-only] [--reset] [--no-browser]
```

### Purpose

Guide the user through local setup, including config, database, API keys, key validation, optional bootstrap index download, and first lookup handoff.

### Interactive flow

```text
Welcome to OpenHaul Guard.
This tool checks carrier records from public sources and stores snapshots locally.

Step 1 of 5: local database
  Create ~/.openhaulguard/ohg.db? [Y/n]

Step 2 of 5: FMCSA WebKey
  Live FMCSA lookups require a free FMCSA developer WebKey.
  Press Enter to open the FMCSA developer login page.
  Or type 'skip' to use offline/bootstrap mode for now.

Step 3 of 5: Socrata/DataHub app token
  DOT DataHub bulk queries work better with a free app token.
  Press Enter to open the token page.
  Or type 'skip'.

Step 4 of 5: bootstrap index
  Download a small MC/DOT lookup index? [Y/n]

Step 5 of 5: validation
  Run ohg doctor now? [Y/n]
```

### Browser behavior

If browser opening is supported, use OS default browser.

If browser opening fails:

```text
I could not open your browser automatically.
Open this URL:
https://mobile.fmcsa.dot.gov/QCDevsite/docs/apiAccess
```

### Required subcommands

```bash
ohg setup --quick
ohg setup --keys-only
ohg setup fmcsa
ohg setup socrata
ohg setup hosted
ohg setup reset
```

## 3. `ohg doctor`

### Command

```bash
ohg doctor [--format table|json|markdown]
```

### Checks

- OHG home directory exists
- Config file readable
- SQLite database exists and migrations are current
- Raw payload directory writable
- FMCSA WebKey present
- FMCSA WebKey validates with a low-cost request
- Socrata app token present, if configured
- Socrata app token validates with a low-cost request
- Bootstrap index present, if configured
- `pdftotext` available, if packet checking is enabled
- MCP server can initialize

### Example human output

```text
OpenHaul Guard doctor

Local database: OK
Raw storage: OK
FMCSA WebKey: OK
Socrata app token: missing, optional
Bootstrap index: OK
PDF extraction: pdftotext not found, packet checks may be limited

Status: usable
```

## 4. `ohg carrier lookup`

### Commands

```bash
ohg carrier lookup --mc <number>
ohg carrier lookup --mx <number>
ohg carrier lookup --ff <number>
ohg carrier lookup --dot <number>
ohg carrier lookup --name <name>
```

### Flags

```text
--force-refresh          Bypass freshness cache
--max-age <duration>     Use cache if younger than duration, default 24h
--offline                Use only local data
--include-raw            Include raw source references in JSON
--save-report <path>     Write report to path
--format <format>        table, json, markdown, html
```

### Required behavior

- Exactly one identifier flag is required.
- MC/MX/FF input is normalized to a numeric docket value and identifier type.
- USDOT is the internal primary identity when resolvable.
- First lookup stores raw payloads and normalized observation.
- Repeat lookup may reuse cache unless stale or forced.

### Example

```bash
ohg carrier lookup --mc 123456 --format markdown
```

### JSON response skeleton

```json
{
  "schema_version": "1.0",
  "lookup": {
    "input_type": "mc",
    "input_value": "123456",
    "resolved_usdot": "1234567",
    "looked_up_at": "2026-04-25T18:30:00Z",
    "mode": "live"
  },
  "carrier": {},
  "freshness": {},
  "risk_assessment": {},
  "sources": [],
  "warnings": []
}
```

## 5. `ohg carrier diff`

### Command

```bash
ohg carrier diff --mc <number> --since <duration|date>
ohg carrier diff --dot <number> --between <date1>..<date2>
```

### Flags

```text
--strict                 Include formatting-only diffs
--field <path>           Limit diff to field prefix
--format <format>        table, json, markdown
```

### Example

```bash
ohg carrier diff --mc 123456 --since 90d --format markdown
```

## 6. `ohg watch`

### Commands

```bash
ohg watch add --mc <number> [--label <text>]
ohg watch add --dot <number> [--label <text>]
ohg watch remove --mc <number>
ohg watch list
ohg watch sync [--all] [--label <text>] [--force-refresh]
ohg watch report [--since 24h]
```

### Acceptance criteria

- Watch entries persist in SQLite.
- Sync performs lookup for each watch item.
- Sync writes observations and risk assessments.
- Report shows material changes and current high flags.

## 7. `ohg packet check`

### Command

```bash
ohg packet check <path> --mc <number>
ohg packet check <path> --dot <number>
```

### Flags

```text
--extract-only           Extract fields without source comparison
--save-extracted <path>  Save extracted fields as JSON
--format <format>        table, json, markdown, html
```

### Behavior

1. Extract text from packet.
2. Extract structured fields.
3. Lookup or load carrier profile.
4. Compare packet fields to public-record fields.
5. Return mismatches and review recommendation.

## 8. `ohg mcp serve`

### Command

```bash
ohg mcp serve [--transport stdio|http] [--host 127.0.0.1] [--port 8798]
```

Default transport: stdio

MCP is specified in `07_MCP_SPEC.md`.

## 9. `ohg config`

### Commands

```bash
ohg config get <key>
ohg config set <key> <value>
ohg config list
ohg config path
```

Allowed config keys:

```text
mode
cache.max_age
sources.fmcsa.enabled
sources.socrata.enabled
sources.mirror.enabled
reports.default_format
mcp.transport
mcp.host
mcp.port
privacy.telemetry
```

Telemetry must default to false.
