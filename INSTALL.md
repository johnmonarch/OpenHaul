# Install

## Requirements

- Go 1.23 or newer
- A shell environment that can run the `ohg` binary
- Optional: an FMCSA WebKey for live carrier lookups
- Optional: `pdftotext` from poppler for PDF packet checks

## Build From Source

```bash
git clone https://github.com/openhaulguard/openhaulguard.git
cd openhaulguard
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
