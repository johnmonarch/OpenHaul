# Security

OpenHaul Guard is a local-first CLI, but it can handle API credentials, carrier packet contents, raw government-source responses, and local SQLite data. Treat bug reports and logs as potentially sensitive.

## Reporting a Vulnerability

Please report suspected vulnerabilities privately.

Preferred path:

1. Open a private GitHub security advisory for this repository if private vulnerability reporting is enabled.
2. If that is unavailable, contact the maintainers before publishing details publicly.
3. Do not open a public issue for suspected credential leaks, auth bypasses, unsafe file handling, report disclosure, or dependency vulnerabilities with an active exploit path.

Include:

- Affected version, tag, or commit SHA
- Operating system and architecture
- Exact command or integration surface involved
- Reproduction steps using sanitized data
- Impact, expected attacker capability, and any workaround
- Whether secrets, carrier packet contents, raw payloads, or local databases may be exposed

Do not include live FMCSA WebKeys, Socrata app tokens, carrier packet contents, customer names, private load data, raw production payloads, screenshots with secrets, or local database files in the initial report.

## Response Expectations

This project is early-stage OSS. Maintainers will aim to:

- Acknowledge credible private reports within a reasonable timeframe.
- Ask for missing reproduction details when needed.
- Prioritize issues that expose credentials, local files, carrier packet contents, or network-accessible API surfaces.
- Release a fix and advisory when a vulnerability affects published releases.
- Credit reporters when requested and appropriate.

Please allow maintainers time to investigate before public disclosure.

## Supported Versions

Before `v1.0.0`, security fixes are applied to `main` and to the latest tagged release when practical. Older preview tags may not receive backports unless the issue is severe and the fix is low-risk.

## Security Model

OpenHaul Guard is designed to run on an operator-controlled workstation or server.

- The CLI stores operational data under `OHG_HOME`, defaulting to `~/.openhaulguard`.
- API credentials are stored in the OS keychain when available, with a local `0600` fallback.
- FMCSA WebKeys are redacted from stored request metadata.
- Telemetry is disabled by default.
- The HTTP API binds to `127.0.0.1` by default and should be token-protected before binding to non-loopback interfaces.
- MCP runs over stdio for local tool integrations.

## Dependency Scanning

The security workflow installs and runs:

```bash
govulncheck ./...
```

It runs on pull requests, pushes to `main`, a weekly schedule, and manual dispatch. Dependabot is configured for Go modules and GitHub Actions so dependency updates can be reviewed as normal pull requests.

## Secret Handling

Rotate credentials if they were pasted into issue reports, logs, screenshots, shell history, generated reports, raw payload directories, or committed files.

For public troubleshooting, prefer:

```bash
ohg doctor --format json
ohg carrier lookup --mc 123456 --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json --format json
```

Review JSON output before sharing it. Remove local paths, tokens, customer-specific labels, and any carrier packet content that should not be public.
