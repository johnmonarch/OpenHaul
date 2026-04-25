# Security

## Reporting a Vulnerability

Please report suspected vulnerabilities privately. If GitHub private vulnerability reporting is enabled for the repository, use that. Otherwise, contact the maintainers before publishing details.

Include:

- Affected version or commit
- Operating system and architecture
- Reproduction steps
- Impact and any known workaround

Do not include live FMCSA WebKeys, Socrata app tokens, carrier packet contents, or other secrets in a report.

## Supported Versions

This project is in early developer preview. Security fixes are applied to the main branch and to the latest tagged release when practical.

## Dependency Scanning

The security workflow installs and runs:

```bash
govulncheck ./...
```

The workflow uses temp-local Go cache paths and runs on pull requests, pushes to `main`, a weekly schedule, and manual dispatch.

## Secret Handling

OpenHaul Guard stores API credentials in the OS keychain when available, with a local `0600` fallback. The CLI redacts FMCSA WebKeys from stored request metadata and does not print secrets in normal command output.

Users should rotate credentials if they were pasted into issue reports, logs, screenshots, shell history, or committed files.
