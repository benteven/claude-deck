#!/bin/sh
set -eu

repo="benteven/claude-deck"
bin_dir="${CLAUDE_DECK_BIN_DIR:-$HOME/.local/bin}"
base_url="${CLAUDE_DECK_BASE_URL:-}"

fail() {
	printf 'claude-deck install: %s\n' "$1" >&2
	exit 1
}

os="${CLAUDE_DECK_OS:-$(uname -s | tr '[:upper:]' '[:lower:]')}"
case "$os" in
darwin | linux) ;;
*) fail "unsupported os: $os" ;;
esac

arch="${CLAUDE_DECK_ARCH:-$(uname -m)}"
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "unsupported architecture: $arch" ;;
esac

version="${CLAUDE_DECK_VERSION:-}"
if [ -z "$version" ]; then
	api="https://api.github.com/repos/$repo/releases/latest"
	version=$(curl -fsSL "$api" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n 1)
	[ -n "$version" ] || fail "could not find the latest release"
fi
version="${version#v}"

if [ -z "$base_url" ]; then
	base_url="https://github.com/$repo/releases/download/v$version"
fi

name="claude-deck_${version}_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSL -o "$tmp/$name" "$base_url/$name" || fail "download failed: $base_url/$name"
curl -fsSL -o "$tmp/checksums.txt" "$base_url/checksums.txt" || fail "download failed: $base_url/checksums.txt"

expected=$(awk -v f="$name" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "no checksum listed for $name"

if command -v sha256sum >/dev/null 2>&1; then
	actual=$(sha256sum "$tmp/$name" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	actual=$(shasum -a 256 "$tmp/$name" | awk '{ print $1 }')
else
	fail "sha256sum or shasum is required"
fi
[ "$expected" = "$actual" ] || fail "checksum mismatch for $name"

tar -xzf "$tmp/$name" -C "$tmp"
mkdir -p "$bin_dir"
install -m 755 "$tmp/${name%.tar.gz}/claude-deck" "$bin_dir/claude-deck"

printf 'installed %s to %s/claude-deck\n' "$version" "$bin_dir"

command -v tmux >/dev/null 2>&1 || printf 'warning: tmux not found. install tmux with your package manager\n' >&2
command -v claude >/dev/null 2>&1 || printf 'warning: claude not found on PATH. install Claude Code before you start sessions\n' >&2
case ":$PATH:" in
*":$bin_dir:"*) ;;
*) printf 'warning: %s is not on your PATH. add it to your shell profile\n' "$bin_dir" >&2 ;;
esac
