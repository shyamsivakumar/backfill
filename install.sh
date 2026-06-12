#!/bin/sh
set -eu

repo="shyamsivakumar/backfill"

fail() {
  echo "error: $*" >&2
  exit 1
}

os="$(uname -s 2>/dev/null || true)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux) os="linux" ;;
  *) fail "unsupported OS: $os" ;;
esac

arch="$(uname -m 2>/dev/null || true)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) fail "unsupported architecture: $arch" ;;
esac

name="backfill_${os}_${arch}.tar.gz"
url="https://github.com/${repo}/releases/latest/download/${name}"

tmp="${TMPDIR:-/tmp}/backfill-install.$$"
mkdir -p "$tmp"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

echo "downloading $url"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$url" -o "$tmp/$name" || fail "download failed"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$tmp/$name" "$url" || fail "download failed"
else
  fail "curl or wget is required"
fi

tar -xzf "$tmp/$name" -C "$tmp" || fail "could not extract archive"

[ -x "$tmp/bf" ] || fail "archive did not contain executable bf"

"$tmp/bf" version >/dev/null 2>&1 || fail "downloaded bf did not run successfully"

if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  bindir="/usr/local/bin"
else
  bindir="$HOME/.local/bin"
  mkdir -p "$bindir" || fail "could not create $bindir"
fi

cp "$tmp/bf" "$bindir/bf" || fail "could not install bf to $bindir"
chmod 755 "$bindir/bf" || fail "could not chmod $bindir/bf"

echo "installed bf to $bindir/bf"

case ":${PATH:-}:" in
  *":$bindir:"*) ;;
  *) echo "add $bindir to your PATH to run bf from any shell" ;;
esac

"$bindir/bf" version
