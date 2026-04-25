#!/bin/sh
set -eu

OWNER="${OHG_GITHUB_OWNER:-johnmonarch}"
REPO="${OHG_GITHUB_REPO:-OpenCarrier}"
BIN_NAME="${OHG_BIN_NAME:-ohg}"
INSTALL_DIR="${OHG_INSTALL_DIR:-/usr/local/bin}"
VERSION="${OHG_VERSION:-latest}"
TMP_DIR="${TMPDIR:-/tmp}/ohg-install.$$"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    darwin) echo "darwin" ;;
    linux) echo "linux" ;;
    msys*|mingw*|cygwin*) echo "windows" ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
  esac
}

download() {
  url="$1"
  out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$out"
  else
    echo "Missing required command: curl or wget" >&2
    exit 1
  fi
}

sha256_file() {
  file="$1"
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
  elif command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
  else
    echo "Missing required command: shasum or sha256sum" >&2
    exit 1
  fi
}

install_binary() {
  src="$1"
  dst="$INSTALL_DIR/$BIN_NAME"
  mkdir -p "$INSTALL_DIR" 2>/dev/null || true
  if [ ! -w "$INSTALL_DIR" ]; then
    if command -v sudo >/dev/null 2>&1; then
      sudo mkdir -p "$INSTALL_DIR"
      sudo install -m 0755 "$src" "$dst"
    else
      echo "Install directory is not writable: $INSTALL_DIR" >&2
      echo "Set OHG_INSTALL_DIR to a user-writable directory." >&2
      exit 1
    fi
  else
    install -m 0755 "$src" "$dst"
  fi
}

if ! command -v tar >/dev/null 2>&1; then
  echo "Missing required command: tar" >&2
  exit 1
fi

OS="$(detect_os)"
ARCH="$(detect_arch)"
EXT="tar.gz"
if [ "$OS" = "windows" ]; then
  EXT="zip"
  if ! command -v unzip >/dev/null 2>&1; then
    echo "Missing required command: unzip" >&2
    exit 1
  fi
fi

mkdir -p "$TMP_DIR"

if [ "$VERSION" = "latest" ]; then
  if ! command -v curl >/dev/null 2>&1; then
    echo "Resolving latest release requires curl. Set OHG_VERSION to a tag." >&2
    exit 1
  fi
  headers="$TMP_DIR/latest.headers"
  curl -fsSLI "https://github.com/$OWNER/$REPO/releases/latest" -o "$headers"
  VERSION="$(sed -n 's/^location: .*\/tag\/\(.*\)\r$/\1/p' "$headers" | tail -n 1)"
  if [ -z "$VERSION" ]; then
    echo "Could not resolve latest release tag. Set OHG_VERSION, for example OHG_VERSION=v0.1.0." >&2
    exit 1
  fi
fi

case "$VERSION" in
  v*)
    RELEASE_TAG="$VERSION"
    ASSET_VERSION="${VERSION#v}"
    ;;
  *)
    RELEASE_TAG="v$VERSION"
    ASSET_VERSION="$VERSION"
    ;;
esac

ASSET="ohg_${ASSET_VERSION}_${OS}_${ARCH}.${EXT}"
BASE_URL="https://github.com/$OWNER/$REPO/releases/download/$RELEASE_TAG"
ARCHIVE="$TMP_DIR/$ASSET"
CHECKSUMS="$TMP_DIR/checksums.txt"

echo "Installing OpenHaul Guard $RELEASE_TAG for $OS/$ARCH"
download "$BASE_URL/$ASSET" "$ARCHIVE"
download "$BASE_URL/checksums.txt" "$CHECKSUMS"

expected="$(awk -v asset="$ASSET" '$2 == asset {print $1}' "$CHECKSUMS")"
if [ -z "$expected" ]; then
  echo "Could not find checksum for $ASSET in checksums.txt" >&2
  exit 1
fi
actual="$(sha256_file "$ARCHIVE")"
if [ "$expected" != "$actual" ]; then
  echo "Checksum verification failed for $ASSET" >&2
  echo "Expected: $expected" >&2
  echo "Actual:   $actual" >&2
  exit 1
fi

EXTRACT_DIR="$TMP_DIR/extract"
mkdir -p "$EXTRACT_DIR"
case "$EXT" in
  tar.gz) tar -xzf "$ARCHIVE" -C "$EXTRACT_DIR" ;;
  zip) unzip -q "$ARCHIVE" -d "$EXTRACT_DIR" ;;
esac

binary="$EXTRACT_DIR/$BIN_NAME"
if [ ! -f "$binary" ]; then
  binary="$EXTRACT_DIR/$BIN_NAME.exe"
fi
if [ ! -f "$binary" ]; then
  echo "Archive did not contain $BIN_NAME" >&2
  exit 1
fi

install_binary "$binary"

echo "Installed: $INSTALL_DIR/$BIN_NAME"
"$INSTALL_DIR/$BIN_NAME" --version || true
echo
echo "Next step:"
echo "  $BIN_NAME setup --quick --yes"
echo
echo "For live FMCSA lookups later:"
echo "  $BIN_NAME setup fmcsa"
