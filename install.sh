#!/usr/bin/env bash
set -e

REPO="lxstig/7zkpxc"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="7zkpxc"

echo "========================================================="
echo "   7zkpxc - Automated Binary Installer"
echo "========================================================="

# Detect OS and Architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Error: Unsupported architecture $ARCH"; exit 1 ;;
esac

echo "-> Detected OS: $OS"
echo "-> Detected Architecture: $ARCH"

# Fetch latest release data from GitHub API
echo "-> Fetching latest release info from GitHub..."
LATEST_TAG=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    echo "Error: Failed to fetch the latest release from $REPO"
    exit 1
fi

VERSION=${LATEST_TAG#v} # Strip 'v' from tag to match goreleaser naming format
echo "-> Found latest version: $LATEST_TAG"

# Determine the download URL
# Format: 7zkpxc_2.9.0_linux_amd64.tar.gz
TARBALL="${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_TAG/$TARBALL"

TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

echo "-> Downloading $TARBALL..."
if ! curl -sL "$DOWNLOAD_URL" -o "$TARBALL"; then
    echo "Error: Failed to download $DOWNLOAD_URL"
    exit 1
fi

echo "-> Extracting..."
tar -xzf "$TARBALL"

if [ ! -f "$BINARY_NAME" ]; then
    echo "Error: Extraction failed or binary not found in archive."
    exit 1
fi

echo "-> Installing to $INSTALL_DIR (requires sudo)"
sudo install -m 755 "$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"

# Clean up
cd - > /dev/null
rm -rf "$TMP_DIR"

echo ""
echo "========================================================="
echo "  Success! $BINARY_NAME $LATEST_TAG is now installed in"
echo "  $INSTALL_DIR/$BINARY_NAME"
echo ""
echo "  You can start using it by typing: $BINARY_NAME --help"
echo "========================================================="
