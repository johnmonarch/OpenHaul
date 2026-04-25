# Watchlist Operations

Watchlist operations are local and pull-based. Add carriers once, run scheduled syncs to create fresh observations, and generate JSON reports for downstream scripts or review queues.

## Setup

Use a dedicated OpenHaul Guard home for unattended jobs:

```bash
export OHG_HOME=/var/lib/openhaulguard
install -d -m 0750 "$OHG_HOME" "$OHG_HOME/reports" "$OHG_HOME/logs"
ohg init
ohg setup fmcsa
```

For a user-level workstation install, use the default `~/.openhaulguard` or pass `--home "$HOME/.openhaulguard"`.

Add watched carriers:

```bash
ohg watch add --mc 123456 --label "priority lane"
ohg watch add --dot 1234567 --label "onboarding"
ohg watch list --format json
```

## JSON Commands

Sync active watchlist entries:

```bash
ohg watch sync --force-refresh --format json
```

The sync response is shaped for job monitoring:

```json
{
  "schema_version": "1.0",
  "generated_at": "2026-04-25T12:00:00Z",
  "synced": 2,
  "failed": 0,
  "results": [],
  "warnings": []
}
```

Generate a change report:

```bash
ohg watch report --since 24h --format json
ohg watch report --since 2026-04-01 --label onboarding --format json
```

The report includes stable counters and per-watch status:

```json
{
  "schema_version": "1.0",
  "report_type": "watch_report",
  "generated_at": "2026-04-25T12:05:00Z",
  "since": "2026-04-24T12:05:00Z",
  "total": 2,
  "changed": 1,
  "unchanged": 1,
  "no_data": 0,
  "items": [
    {
      "watch_id": 1,
      "identifier_type": "mc",
      "identifier_value": "123456",
      "label": "priority lane",
      "usdot_number": "1234567",
      "status": "changed",
      "observation_count": 2,
      "last_observed_at": "2026-04-25T12:00:00Z"
    }
  ]
}
```

Export the active watchlist for backup or dashboards:

```bash
ohg watch export --format json
```

## Script Pattern

Use one file per run because JSON output is pretty-printed and may span multiple lines:

```bash
#!/bin/sh
set -eu

export OHG_HOME="${OHG_HOME:-/var/lib/openhaulguard}"
REPORT_DIR="$OHG_HOME/reports"
LOG_DIR="$OHG_HOME/logs"
ts="$(date -u +%Y%m%dT%H%M%SZ)"

mkdir -p "$REPORT_DIR" "$LOG_DIR"

ohg watch sync --force-refresh --format json \
  > "$REPORT_DIR/watch-sync-$ts.json" \
  2>> "$LOG_DIR/watch-sync.err"

ohg watch report --since 24h --format json \
  > "$REPORT_DIR/watch-report-$ts.json" \
  2>> "$LOG_DIR/watch-report.err"
```

With `jq`, fail a job when sync failures occur:

```bash
jq -e '.failed == 0' "$REPORT_DIR/watch-sync-$ts.json" >/dev/null
```

Detect whether a report has changes:

```bash
jq -e '.changed > 0' "$REPORT_DIR/watch-report-$ts.json" >/dev/null
```

## Cron

Example crontab entry for a system account:

```cron
SHELL=/bin/sh
PATH=/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin
OHG_HOME=/var/lib/openhaulguard

15 * * * * ts=$(date -u +\%Y\%m\%dT\%H\%M\%SZ); mkdir -p "$OHG_HOME/reports" "$OHG_HOME/logs"; ohg watch sync --force-refresh --format json > "$OHG_HOME/reports/watch-sync-$ts.json" 2>> "$OHG_HOME/logs/watch-sync.err"
20 * * * * ts=$(date -u +\%Y\%m\%dT\%H\%M\%SZ); mkdir -p "$OHG_HOME/reports" "$OHG_HOME/logs"; ohg watch report --since 24h --format json > "$OHG_HOME/reports/watch-report-$ts.json" 2>> "$OHG_HOME/logs/watch-report.err"
```

Cron treats `%` specially, so date format percent signs must be escaped as `\%`.

## systemd

Create `/etc/systemd/system/ohg-watch-sync.service`:

```ini
[Unit]
Description=OpenHaul Guard watchlist sync
Wants=network-online.target
After=network-online.target

[Service]
Type=oneshot
User=openhaul
Group=openhaul
Environment=OHG_HOME=/var/lib/openhaulguard
ExecStart=/bin/sh -c 'mkdir -p "$OHG_HOME/reports" "$OHG_HOME/logs"; ts=$(date -u +%%Y%%m%%dT%%H%%M%%SZ); /usr/local/bin/ohg watch sync --force-refresh --format json > "$OHG_HOME/reports/watch-sync-$ts.json" 2>> "$OHG_HOME/logs/watch-sync.err"'
```

Create `/etc/systemd/system/ohg-watch-sync.timer`:

```ini
[Unit]
Description=Run OpenHaul Guard watchlist sync hourly

[Timer]
OnCalendar=hourly
Persistent=true
RandomizedDelaySec=5m

[Install]
WantedBy=timers.target
```

Enable it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ohg-watch-sync.timer
systemctl list-timers ohg-watch-sync.timer
```

Use a second service/timer for `ohg watch report --since 24h --format json` if reports should be generated on a different cadence than syncs.

## launchd

Create `~/Library/LaunchAgents/org.openhaulguard.watch-sync.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>org.openhaulguard.watch-sync</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/sh</string>
    <string>-lc</string>
    <string>mkdir -p "$HOME/.openhaulguard/reports" "$HOME/.openhaulguard/logs"; ts=$(date -u +%Y%m%dT%H%M%SZ); /opt/homebrew/bin/ohg watch sync --force-refresh --format json &gt; "$HOME/.openhaulguard/reports/watch-sync-$ts.json" 2&gt;&gt; "$HOME/.openhaulguard/logs/watch-sync.err"</string>
  </array>
  <key>StartInterval</key>
  <integer>3600</integer>
  <key>RunAtLoad</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/openhaulguard-watch-sync.out</string>
  <key>StandardErrorPath</key>
  <string>/tmp/openhaulguard-watch-sync.err</string>
</dict>
</plist>
```

Load it:

```bash
launchctl load ~/Library/LaunchAgents/org.openhaulguard.watch-sync.plist
launchctl list | grep openhaulguard
```

For Intel Homebrew or a manual install, replace `/opt/homebrew/bin/ohg` with the actual binary path from `command -v ohg`.

## Operational Notes

- Run `ohg doctor --format json` after installation and after credential changes.
- Keep `OHG_HOME` readable only by the service user when reports or labels may be sensitive.
- Rotate FMCSA credentials if logs, shell history, or support bundles may contain secrets.
- Keep report retention explicit, for example `find "$OHG_HOME/reports" -name 'watch-*.json' -mtime +30 -delete`.
- Use `watch export --format json` before moving hosts so the active watchlist can be reviewed or restored manually.
