# Install

## Recommended Install

### One-Line Installer

Install the latest binary with:

```bash
curl -fsSL https://github.com/johnmonarch/OpenCarrier/releases/latest/download/install.sh | sh
```

The installer:

- Detects macOS/Linux and `amd64`/`arm64`
- Downloads the matching GitHub Release archive
- Verifies `checksums.txt`
- Installs `ohg` into `/usr/local/bin` by default
- Prints the next setup command

For a user-local install:

```bash
mkdir -p "$HOME/.local/bin"
curl -fsSL https://github.com/johnmonarch/OpenCarrier/releases/latest/download/install.sh \
  | OHG_INSTALL_DIR="$HOME/.local/bin" sh
```

Install a specific release:

```bash
curl -fsSL https://github.com/johnmonarch/OpenCarrier/releases/latest/download/install.sh \
  | OHG_VERSION=v0.1.0 sh
```

### Homebrew

```bash
brew tap johnmonarch/openhaulguard
brew install ohg
```

The release workflow can update that tap when `HOMEBREW_TAP_GITHUB_TOKEN` is configured in GitHub Actions.

### Linux Packages

Tagged releases also produce Linux packages through GoReleaser:

- `.deb` for Debian/Ubuntu
- `.rpm` for Fedora/RHEL-compatible systems
- `.apk` for Alpine

Example:

```bash
sudo dpkg -i ohg_*_linux_amd64.deb
```

## Requirements

- A shell environment that can run the `ohg` binary
- Optional: an FMCSA WebKey for live carrier lookups
- Optional: `pdftotext` from poppler for PDF packet checks

## Build From Source

Source builds require Go 1.23 or newer.

```bash
git clone https://github.com/johnmonarch/OpenCarrier.git
cd OpenCarrier
go build -o ohg ./cmd/ohg
```

Move the binary somewhere on your `PATH` if desired:

```bash
install -m 0755 ohg /usr/local/bin/ohg
```

For a user-local install:

```bash
mkdir -p "$HOME/.local/bin"
install -m 0755 ohg "$HOME/.local/bin/ohg"
```

## First Run

```bash
ohg setup --quick --yes
ohg doctor
```

Quick setup creates the local OpenHaul Guard home directory, config file, SQLite database, raw payload directory, reports directory, and logs directory. It does not configure live government API credentials.

## Live FMCSA Lookup

```bash
ohg setup fmcsa
ohg carrier lookup --mc 123456
```

`ohg setup fmcsa` validates the WebKey before storing it. Secrets are stored in the OS keychain when available, with a local `0600` fallback.

## Release Archives

Tagged releases are packaged by GoReleaser for:

- `darwin/amd64`
- `darwin/arm64`
- `linux/amd64`
- `linux/arm64`
- `windows/amd64`
- `windows/arm64`

Non-Windows archives are `tar.gz`; Windows archives are `zip`. Each release includes `checksums.txt`.

Current module path note: `go.mod` uses `github.com/openhaulguard/openhaulguard` as the intended long-term module path, while this repository currently lives at `github.com/johnmonarch/OpenCarrier`. Use release binaries, Homebrew, packages, or local source builds until the canonical repo path is finalized.
