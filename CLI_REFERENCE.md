# CLI Reference

Binary:

```bash
ohg
```

## Global Flags

```text
--config string   Path to config file
--debug           Enable debug logs
--format string   Output format: table, json, markdown (default "table")
--home string     Override OHG home directory
--no-color        Disable terminal colors
--offline         Do not call network sources
--verbose         Enable verbose output
--version         Print version
--yes             Accept safe defaults in prompts
```

## Commands

```text
ohg setup
ohg init
ohg doctor
ohg carrier lookup
ohg carrier diff
ohg watch add
ohg watch remove
ohg watch list
ohg watch sync
ohg watch report
ohg watch export
ohg mirror status
ohg mirror build
ohg mirror import
ohg config path
ohg config list
ohg config get
ohg config set
ohg mcp serve
ohg packet extract
ohg packet check
```

## setup

```bash
ohg setup [--quick] [--no-browser]
ohg setup fmcsa [--no-browser]
ohg setup socrata [--no-browser]
ohg init
```

`setup` runs the guided local setup and saves progress in the local setup state, so rerunning it can resume safely. It creates local config and SQLite storage without requiring API credentials. `ohg init` is a quick alias for local setup. `setup fmcsa` validates and stores a WebKey. `setup socrata` stores an app token for future data-source work.

## doctor

```bash
ohg doctor --format table
ohg doctor --format json
ohg doctor --format markdown
```

Checks local directories, config, database migrations, credential presence, `pdftotext`, and MCP availability.

## carrier lookup

```bash
ohg carrier lookup --mc <number>
ohg carrier lookup --mx <number>
ohg carrier lookup --ff <number>
ohg carrier lookup --dot <number>
ohg carrier lookup --name <name>
```

Exactly one identifier flag is required.

Flags:

```text
--dot string           USDOT number
--ff string            Freight forwarder docket number
--force-refresh        Bypass freshness cache
--max-age string       Use cache if younger than duration (default "24h")
--mc string            Motor Carrier docket number
--mx string            Mexico docket number
--name string          Carrier name
--save-report string   Write report to path
```

Examples:

```bash
ohg carrier lookup --mc 123456 --format table
ohg carrier lookup --dot 1234567 --format json
ohg carrier lookup --mc 123456 --save-report report.md --format markdown
ohg carrier lookup --mc 123456 --offline
```

## carrier diff

```bash
ohg carrier diff --mc <number> [--since 90d]
ohg carrier diff --dot <number> [--since 2026-01-01]
```

Flags:

```text
--dot string     USDOT number
--mc string      Motor Carrier docket number
--since string   Duration or YYYY-MM-DD start date (default "90d")
--strict         Include formatting-only diffs
```

Diff compares stored local observations. It needs at least two observations for the carrier to show changes.

## watch

```bash
ohg watch add --mc <number> [--label <text>]
ohg watch add --dot <number> [--label <text>]
ohg watch remove --mc <number>
ohg watch remove --dot <number>
ohg watch list
ohg watch sync [--force-refresh]
ohg watch report [--since 24h] [--label <text>] [--format table|json|markdown]
ohg watch export [--format table|json|markdown]
```

`watch sync` runs lookup for active watchlist entries and updates `last_synced_at` when successful.
`watch report` summarizes active watchlist changes from local observations since the requested duration or date.
`watch export` writes active watchlist entries for reports, backups, or scheduled jobs.

## mirror

```bash
ohg mirror status
ohg mirror build <company-census-json> [--output <path>]
ohg mirror import <path>
```

`mirror build` converts DOT/DataHub Company Census JSON rows into the local mirror format. If `--output` is omitted, it writes to the configured local mirror path. `mirror import` installs an existing local JSON bootstrap mirror at the configured mirror path. When no FMCSA WebKey is configured and the carrier is not already cached, `carrier lookup` can use this local mirror and marks the lookup mode as `mirror`.

Build flags:

```text
--attribution string        Attribution text for the mirror
--generated-at string       Mirror generated timestamp as RFC3339 or YYYY-MM-DD
--output string             Write mirror JSON to path, or '-' for stdout
--source-timestamp string   Source data timestamp or date
```

The initial mirror schema is:

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-04-25T00:00:00Z",
  "source_timestamp": "2026-04-24",
  "carriers": []
}
```

## config

```bash
ohg config path
ohg config list
ohg config get <key>
ohg config set <key> <value>
```

Supported non-secret keys:

```text
mode
cache.max_age
reports.default_format
mcp.transport
mcp.host
privacy.telemetry
sources.mirror.enabled
sources.mirror.local_path
sources.mirror.url
```

Credential keys accepted by `config set`:

```text
fmcsa.web_key
socrata.app_token
```

## mcp

```bash
ohg mcp serve
```

Runs the developer-preview MCP JSON-RPC server over stdio.

Supported JSON-RPC methods:

```text
initialize
notifications/initialized
tools/list
tools/call
```

Supported tools:

```text
carrier_lookup
carrier_diff
packet_extract
packet_check
```

## packet

```bash
ohg packet extract <path>
ohg packet check <path> --mc <number>
ohg packet check <path> --dot <number>
```

Flags:

```text
--dot string   USDOT number
--mc string    Motor Carrier docket number
```

Packet extract accepts text files, extensionless text files, and text-based PDFs when `pdftotext` is installed. It writes extracted carrier, contact, insurance, certificate holder, remit-to/payee, and factoring fields as table, JSON, or Markdown.

Packet check uses the same extraction path, runs a carrier lookup, and writes a table, JSON, or Markdown comparison report.

## Exit Codes

```text
0   success
1   generic error
2   invalid arguments
3   setup incomplete
4   FMCSA authentication missing or invalid
5   source unavailable or rate-limited
6   carrier not found
7   local database error
8   packet parse failure
10  offline cache miss
```
