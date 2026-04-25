# v0.1.0 Release Checklist

This runbook is for maintainers publishing the first OpenHaul Guard preview release from `johnmonarch/OpenHaul`.

## Release Scope

`v0.1.0` should prove that users can install `ohg`, run local setup, perform fixture-backed and live lookups, use watchlist JSON outputs, and install through Homebrew or Linux packages.

## Preflight

Start from a clean, up-to-date `main` branch:

```bash
git checkout main
git pull --ff-only
git status --short
```

Run local checks:

```bash
gofmt -l .
go test ./...
go build -trimpath -o /tmp/ohg ./cmd/ohg
sh -n install.sh
actionlint .github/workflows/release.yml
goreleaser check
```

If `actionlint` or `goreleaser` is not installed locally, rely on the `Release` workflow validation job before tagging.

## Homebrew Tap and Token

The GoReleaser config publishes the Homebrew formula to:

```text
johnmonarch/homebrew-openhaulguard
```

Confirm the tap exists and has a `Formula/` directory:

```bash
gh repo view johnmonarch/homebrew-openhaulguard \
  --json nameWithOwner,defaultBranchRef \
  --jq '{repo: .nameWithOwner, default_branch: .defaultBranchRef.name}'

gh api repos/johnmonarch/homebrew-openhaulguard/contents/Formula \
  --jq '.[].name'
```

Confirm the release repository has the required secret:

```bash
gh secret list --repo johnmonarch/OpenHaul | grep '^HOMEBREW_TAP_GITHUB_TOKEN'
```

`HOMEBREW_TAP_GITHUB_TOKEN` must be a fine-grained token or GitHub App token with contents read/write access to `johnmonarch/homebrew-openhaulguard`. The default `GITHUB_TOKEN` cannot push to a separate tap repository.

If validating a local token before saving it as a secret, do not print the token. Use it only through `GH_TOKEN`:

```bash
GH_TOKEN="$HOMEBREW_TAP_GITHUB_TOKEN" \
  gh api repos/johnmonarch/homebrew-openhaulguard \
  --jq '.permissions.push'
```

The command should print `true`.

## Tag and Publish

Create and push an annotated tag:

```bash
git tag -a v0.1.0 -m "OpenHaul Guard v0.1.0"
git push origin v0.1.0
```

Watch the release workflow:

```bash
gh run list --repo johnmonarch/OpenHaul --workflow Release --limit 5
gh run watch --repo johnmonarch/OpenHaul
```

The workflow should:

1. Run installer syntax validation.
2. Run `goreleaser check`.
3. Validate the tag format.
4. Fail early if `HOMEBREW_TAP_GITHUB_TOKEN` is missing.
5. Publish GitHub release artifacts.
6. Push the Homebrew formula update to the tap.

## Artifact Checks

Confirm the GitHub release includes:

- `install.sh`
- `checksums.txt`
- `ohg_0.1.0_darwin_amd64.tar.gz`
- `ohg_0.1.0_darwin_arm64.tar.gz`
- `ohg_0.1.0_linux_amd64.tar.gz`
- `ohg_0.1.0_linux_arm64.tar.gz`
- `ohg_0.1.0_windows_amd64.zip`
- `ohg_0.1.0_windows_arm64.zip`
- `.deb`, `.rpm`, and `.apk` packages for Linux

List assets with:

```bash
gh release view v0.1.0 --repo johnmonarch/OpenHaul --json assets \
  --jq '.assets[].name'
```

## Smoke Tests

Installer:

```bash
tmp="$(mktemp -d)"
curl -fsSL https://github.com/johnmonarch/OpenHaul/releases/download/v0.1.0/install.sh \
  | OHG_VERSION=v0.1.0 OHG_INSTALL_DIR="$tmp/bin" sh
"$tmp/bin/ohg" --version
"$tmp/bin/ohg" setup --help
```

Fixture-backed lookup:

```bash
OHG_HOME="$tmp/home" "$tmp/bin/ohg" init
OHG_HOME="$tmp/home" "$tmp/bin/ohg" carrier lookup --mc 123456 \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format json
```

Watchlist JSON:

```bash
OHG_HOME="$tmp/home" "$tmp/bin/ohg" watch add --mc 123456 --label release-smoke
OHG_HOME="$tmp/home" "$tmp/bin/ohg" watch sync \
  --fixture examples/fixtures/fmcsa_qcmobile/fmcsa_qcmobile_carrier_valid.json \
  --format json
OHG_HOME="$tmp/home" "$tmp/bin/ohg" watch report --since 24h --format json
OHG_HOME="$tmp/home" "$tmp/bin/ohg" watch export --format json
```

Homebrew:

```bash
brew update
brew install johnmonarch/openhaulguard/ohg
ohg --version
ohg setup --help
brew test johnmonarch/openhaulguard/ohg
```

Linux packages, on matching Linux hosts:

```bash
sudo dpkg -i ohg_0.1.0_linux_amd64.deb
ohg --version

sudo rpm -Uvh ohg_0.1.0_linux_amd64.rpm
ohg --version

sudo apk add --allow-untrusted ohg_0.1.0_linux_amd64.apk
ohg --version
```

## If Publishing Fails

- If the tap update fails because of auth, fix `HOMEBREW_TAP_GITHUB_TOKEN` and rerun the failed workflow job if GitHub allows it. Otherwise delete the failed release and tag only if no assets were published successfully and no users could have consumed it.
- If GitHub release assets are incomplete, do not announce the release. Fix the release config and republish from a new tag if the broken tag was public for any meaningful time.
- If Homebrew is the only failed step and GitHub release assets are good, manually open a tap PR with the formula update or rerun after fixing the token.

## Announcement Notes

Call out that `v0.1.0` is a developer preview. Mention supported OS/architecture pairs, install methods, local-first storage, optional FMCSA WebKey setup, and where to report security issues.
