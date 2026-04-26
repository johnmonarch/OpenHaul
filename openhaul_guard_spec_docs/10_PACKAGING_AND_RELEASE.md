<!--
OpenHaul Guard OSS specification bundle
Generated: 2026-04-25
Audience: coding agents and human maintainers
Primary implementation language: Go
-->
# 10 Packaging and Release Specification

## 1. Release artifacts

Each release must include:

```text
ohg_darwin_arm64.tar.gz
ohg_darwin_amd64.tar.gz
ohg_linux_amd64.tar.gz
ohg_linux_arm64.tar.gz
ohg_windows_amd64.zip
checksums.txt
SBOM if available
```

## 2. Installation paths

### Homebrew

Preferred macOS/Linux install:

```bash
brew install openhaulguard/tap/ohg
```

### One-line installer

```bash
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/latest/download/install.sh | sh
```

Installer requirements:

- Detect OS and architecture.
- Download from GitHub Releases.
- Verify checksum.
- Install to `/usr/local/bin` or prompt for user-local bin.
- Print next command: `ohg setup`.

### Manual download

GitHub Releases must include manual install instructions.

### Docker

Post-MVP or advanced MVP:

```bash
docker run --rm -it \
  -v $HOME/.openhaulguard:/data \
  ghcr.io/openhaulguard/ohg:latest setup
```

Docker image should use non-root user.

## 3. Versioning

Use semantic versioning:

```text
0.1.0 initial developer preview
0.2.0 first usable CLI lookup
0.3.0 watchlist and diff
0.4.0 packet checker
0.5.0 MCP server
1.0.0 stable schemas and CLI
```

## 4. Backward compatibility

Before 1.0:

- CLI may change, but release notes must call breaking changes out.
- JSON schemas may change, but include `schema_version`.

After 1.0:

- Do not break CLI flags without deprecation.
- Do not remove JSON fields without one minor release deprecation.
- Database migrations must be forward-only and tested.

## 5. GitHub Actions

Required workflows:

```text
ci.yml              test, lint, build
release.yml         goreleaser release
security.yml        govulncheck and dependency audit
docs.yml            markdown lint and link check
```

## 6. GoReleaser

Use GoReleaser for cross-platform binaries.

Requirements:

- Embed version, commit, build date.
- Generate checksums.
- Publish GitHub release.
- Generate Homebrew formula update.

## 7. License

Recommended license:

Apache-2.0 for broad adoption.

Alternative:

AGPL-3.0 if the strategic goal is to force network-service modifications to be shared. This may reduce enterprise adoption.

Spec recommendation:

Use Apache-2.0 for OSS CLI and keep hosted differentiation through managed data, history, scale, and support rather than license friction.

## 8. Documentation

Required docs in repo:

```text
README.md
INSTALL.md
CONFIGURATION.md
DATA_SOURCES.md
CLI_REFERENCE.md
MCP.md
SKILL.md
CONTRIBUTING.md
SECURITY.md
```

## 9. First-run user experience

After install, any command that requires setup should print:

```text
OpenHaul Guard is not set up yet.
Run: ohg setup
```

After setup completes, print:

```text
Setup complete.
Try your first lookup:
  ohg carrier lookup --mc 123456
```

## 10. Telemetry

Telemetry must be disabled by default.

If introduced later:

- Explicit opt-in only.
- No carrier identifiers without explicit consent.
- No packet contents.
- No API keys.
- Clear `ohg telemetry off` command.
