# Install

## Recommended Install

### One-Line Installer

Install the latest binary with:

```bash
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/latest/download/install.sh | sh
```

The installer:

- Detects macOS/Linux and `amd64`/`arm64`
- Downloads the matching GitHub Release archive
- Verifies `checksums.txt`
- Installs `ohg` into `/usr/local/bin` by default, using `sudo` if needed
- Falls back to `$HOME/.local/bin` when `/usr/local/bin` is not writable and `sudo` is unavailable
- Prints a `PATH` hint when the install directory is not already on `PATH`
- Prints the next setup command

For a user-local install that never needs `sudo`:

```bash
mkdir -p "$HOME/.local/bin"
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/latest/download/install.sh \
  | OHG_INSTALL_DIR="$HOME/.local/bin" sh
```

Install the `v0.1.0` release explicitly:

```bash
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/download/v0.1.0/install.sh \
  | OHG_VERSION=v0.1.0 sh
```

### Homebrew

```bash
brew tap johnmonarch/openhaulguard
brew install ohg
```

Or install the formula directly:

```bash
brew install johnmonarch/openhaulguard/ohg
```

The Homebrew tap repository is `johnmonarch/homebrew-openhaulguard`. The release workflow updates that tap when `HOMEBREW_TAP_GITHUB_TOKEN` is configured in GitHub Actions.

### Linux Packages

Tagged releases also produce Linux packages through GoReleaser:

- `.deb` for Debian/Ubuntu
- `.rpm` for Fedora/RHEL-compatible systems
- `.apk` for Alpine

Example:

```bash
sudo dpkg -i ohg_*_linux_amd64.deb
```

Package-manager examples:

```bash
sudo dpkg -i ohg_0.1.0_linux_amd64.deb
sudo rpm -Uvh ohg_0.1.0_linux_amd64.rpm
sudo apk add --allow-untrusted ohg_0.1.0_linux_amd64.apk
```

## Requirements

- A shell environment that can run the `ohg` binary
- Optional: an FMCSA WebKey for live carrier lookups
- Optional: `pdftotext` from poppler for PDF packet checks

## Build From Source

Source builds require Go 1.23 or newer.

```bash
git clone https://github.com/johnmonarch/OpenHaul.git
cd OpenHaul
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
ohg setup
ohg doctor
```

Guided setup creates the local OpenHaul Guard home directory, config file, SQLite database, raw payload directory, reports directory, and logs directory. It saves setup progress locally, so rerunning `ohg setup` can continue safely. It does not configure live government API credentials.

For scripts or a fast local bootstrap, use:

```bash
ohg init
```

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

Current module path note: `go.mod` uses `github.com/openhaulguard/openhaulguard` as the intended long-term module path, while this repository currently lives at `github.com/johnmonarch/OpenHaul`. Use release binaries, Homebrew, packages, or local source builds until the canonical repo path is finalized.

## Maintainer Release Checklist

Use this checklist for the `v0.1.0` release path.

1. Confirm the Homebrew tap repository exists at `johnmonarch/homebrew-openhaulguard` with a `main` branch and a `Formula/` directory.
2. Create a repository secret named `HOMEBREW_TAP_GITHUB_TOKEN` in `johnmonarch/OpenHaul`. Use a fine-grained token or GitHub App token that can write contents to `johnmonarch/homebrew-openhaulguard`; the default `GITHUB_TOKEN` cannot push to a different repository.
3. Run local release checks before tagging:

```bash
sh -n install.sh
actionlint .github/workflows/release.yml
goreleaser check
```

4. Tag and push the release:

```bash
git checkout main
git pull --ff-only
git tag -a v0.1.0 -m "OpenHaul Guard v0.1.0"
git push origin v0.1.0
```

5. Watch the `Release` workflow. It validates the GoReleaser config first, checks the tag format and `HOMEBREW_TAP_GITHUB_TOKEN`, then publishes the GitHub release and Homebrew formula.
6. Confirm the GitHub release includes `install.sh`, `checksums.txt`, archives for all supported OS/architecture pairs, and `.deb`, `.rpm`, and `.apk` packages for Linux.
7. Smoke test the release installer:

```bash
tmp="$(mktemp -d)"
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/download/v0.1.0/install.sh \
  | OHG_VERSION=v0.1.0 OHG_INSTALL_DIR="$tmp/bin" sh
"$tmp/bin/ohg" --version
"$tmp/bin/ohg" setup --help
```

8. Smoke test Homebrew:

```bash
brew update
brew install johnmonarch/openhaulguard/ohg
ohg --version
ohg setup --help
```
