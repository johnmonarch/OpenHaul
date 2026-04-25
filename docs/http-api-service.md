# HTTP API Service Operation

`ohg serve` is intended for same-host backend integrations. Bind to loopback unless the machine is on a trusted private network and token auth is configured.

## Local Start

```bash
export OHG_HOME=/var/lib/openhaulguard
export OHG_API_TOKEN="$(openssl rand -base64 32)"
ohg serve --listen 127.0.0.1:8787
```

Check health:

```bash
curl -s http://127.0.0.1:8787/health
```

Call a protected endpoint:

```bash
curl -s http://127.0.0.1:8787/v1/carrier/lookup \
  -H "Authorization: Bearer $OHG_API_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"identifier_type":"mc","identifier_value":"123456","max_age":"24h"}'
```

## systemd

Create a dedicated service user and local state directory:

```bash
sudo useradd --system --home /var/lib/openhaulguard --shell /usr/sbin/nologin openhaul
sudo install -d -o openhaul -g openhaul -m 0750 /var/lib/openhaulguard
```

Store the API token in an environment file readable only by root:

```bash
sudo install -m 0600 /dev/null /etc/openhaulguard-api.env
sudo sh -c 'printf "OHG_API_TOKEN=%s\n" "$(openssl rand -base64 32)" > /etc/openhaulguard-api.env'
```

Create `/etc/systemd/system/ohg-api.service`:

```ini
[Unit]
Description=OpenHaul Guard local HTTP API
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=openhaul
Group=openhaul
Environment=OHG_HOME=/var/lib/openhaulguard
EnvironmentFile=/etc/openhaulguard-api.env
ExecStart=/usr/local/bin/ohg serve --listen 127.0.0.1:8787
Restart=on-failure
RestartSec=5s
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/openhaulguard

[Install]
WantedBy=multi-user.target
```

Enable it:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now ohg-api.service
systemctl status ohg-api.service
```

## launchd

Create `~/Library/LaunchAgents/org.openhaulguard.api.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>org.openhaulguard.api</string>
  <key>ProgramArguments</key>
  <array>
    <string>/opt/homebrew/bin/ohg</string>
    <string>serve</string>
    <string>--listen</string>
    <string>127.0.0.1:8787</string>
  </array>
  <key>EnvironmentVariables</key>
  <dict>
    <key>OHG_HOME</key>
    <string>/Users/your-user/.openhaulguard</string>
    <key>OHG_API_TOKEN</key>
    <string>replace-with-local-token</string>
  </dict>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/tmp/openhaulguard-api.out</string>
  <key>StandardErrorPath</key>
  <string>/tmp/openhaulguard-api.err</string>
</dict>
</plist>
```

Load it:

```bash
launchctl load ~/Library/LaunchAgents/org.openhaulguard.api.plist
launchctl list | grep openhaulguard
```

For Intel Homebrew or a manual install, replace `/opt/homebrew/bin/ohg` with the path from `command -v ohg`. Replace `/Users/your-user/.openhaulguard` with the real user home path; launchd does not expand `$HOME` inside `EnvironmentVariables`.

## Backend Integration Notes

- Call the API from server-side code, not directly from a browser.
- Do not expose FMCSA or Socrata credentials to frontend clients.
- Keep `OHG_HOME` on a disk path readable only by the service account.
- Use `OHG_API_TOKEN` before binding to any non-loopback interface.
- Put nginx, Caddy, or another reverse proxy in front only when you need normal infrastructure controls such as TLS, network ACLs, or centralized logs.
- Webhooks are intentionally not included in the OSS service. Use scheduled `watch sync`, `watch report --format json`, and `watch export --format json` jobs for pull-based automation.
