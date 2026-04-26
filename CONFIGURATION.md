# Configuration

OpenHaul Guard stores local state under `~/.openhaulguard` by default.

## Paths

Default paths:

```text
home:        ~/.openhaulguard
config:      ~/.openhaulguard/config.toml
database:    ~/.openhaulguard/ohg.db
raw data:    ~/.openhaulguard/raw
reports:     ~/.openhaulguard/reports
logs:        ~/.openhaulguard/logs
```

Override paths with flags:

```bash
ohg --home /path/to/home doctor
ohg --config /path/to/config.toml config list
```

Override paths with environment variables:

```text
OHG_HOME
OHG_CONFIG
OHG_DB_PATH
OHG_MODE
```

## Config Commands

```bash
ohg config path
ohg config list
ohg config get <key>
ohg config set <key> <value>
```

Current configurable keys:

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
fmcsa.web_key
socrata.app_token
```

`fmcsa.web_key` and `socrata.app_token` are stored through the credential store, not written to `config.toml`.

## Defaults

```toml
mode = "local"
db_path = "~/.openhaulguard/ohg.db"
raw_dir = "~/.openhaulguard/raw"
reports_dir = "~/.openhaulguard/reports"
log_dir = "~/.openhaulguard/logs"

[cache]
max_age = "24h"

[sources.fmcsa]
enabled = true

[sources.socrata]
enabled = true

[sources.mirror]
enabled = true
local_path = "~/.openhaulguard/mirror/carriers.json"

[reports]
default_format = "table"

[mcp]
transport = "stdio"
host = "127.0.0.1"
port = 8798

[privacy]
telemetry = false
```

OpenHaul Guard does not currently operate a hosted bootstrap mirror or project domain. Build or import a local mirror with `ohg mirror build` or `ohg mirror import`. Leave `sources.mirror.url` and `sources.mirror.checksum_url` unset unless you run your own mirror.

The current CLI does not send telemetry.
