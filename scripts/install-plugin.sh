#!/bin/sh
# Installs docker-nutrition as a Docker CLI plugin so `docker nutrition IMAGE` works.
# Usage: curl -fsSL https://raw.githubusercontent.com/khimananda/docker-nutrition-facts/main/scripts/install-plugin.sh | sh
# (Yes, that is curl | sh. The label would dock you for it. Read the script first.)
set -eu

REPO="khimananda/docker-nutrition-facts"
VERSION="${VERSION:-latest}"
DEST="${DOCKER_CONFIG:-$HOME/.docker}/cli-plugins"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p')
fi
[ -n "$VERSION" ] || { echo "could not resolve the latest release" >&2; exit 1; }

file="docker-nutrition_${VERSION#v}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$VERSION"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading $file ($VERSION)"
curl -fsSL -o "$tmp/$file" "$base/$file"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"
(cd "$tmp" && grep " $file\$" checksums.txt | { command -v sha256sum >/dev/null && sha256sum -c - || shasum -a 256 -c -; })

tar -xzf "$tmp/$file" -C "$tmp" docker-nutrition
mkdir -p "$DEST"
install -m 0755 "$tmp/docker-nutrition" "$DEST/docker-nutrition"
echo "Installed to $DEST/docker-nutrition. Try: docker nutrition nginx:latest"
