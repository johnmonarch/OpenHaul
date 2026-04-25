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
ohg doctor
ohg carrier lookup
ohg carrier diff
ohg watch add
ohg watch list
ohg watch sync
ohg config path
ohg config list
ohg config get
ohg config set
ohg mcp serve
ohg packet check
```

## setup

```bash
ohg setup [--quick] [--no-browser]
ohg setup fmcsa [--no-browser]
ohg setup socrata [--no-browser]
```

`setup --quick` creates local config and SQLite storage without API credentials. `setup fmcsa` validates and stores a WebKey. `setup socrata` stores an app token for future data-source work.

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
ohg watch list
ohg watch sync [--force-refresh]
```

`watch sync` runs lookup for active watchlist entries and updates `last_synced_at` when successful.

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
```

## packet

```bash
ohg packet check <path> --mc <number>
ohg packet check <path> --dot <number>
```

Flags:

```text
--dot string   USDOT number
--mc string    Motor Carrier docket number
```

Packet check accepts text files, extensionless text files, and text-based PDFs when `pdftotext` is installed. It extracts packet fields, runs a carrier lookup, and writes a table, JSON, or Markdown comparison report.

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
