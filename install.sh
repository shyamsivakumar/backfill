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

config_dir="$HOME/.backfill"
config_file="$config_dir/config.json"
if { [ "${BACKFILL_API+x}" ] || [ "${BACKFILL_DEVICE+x}" ]; } && [ ! -e "$config_file" ]; then
  api="${BACKFILL_API-}"
  device="${BACKFILL_DEVICE-}"

  case "$api" in
    *\"* | *\\*)
      echo "warning: BACKFILL_API contains a double quote or backslash; not writing $config_file" >&2
      api_bad=1
      ;;
    *)
      api_bad=0
      ;;
  esac

  case "$device" in
    *\"* | *\\*)
      echo "warning: BACKFILL_DEVICE contains a double quote or backslash; not writing $config_file" >&2
      device_bad=1
      ;;
    *)
      device_bad=0
      ;;
  esac

  if [ "$api_bad" -eq 0 ] && [ "$device_bad" -eq 0 ]; then
    mkdir -p "$config_dir" || fail "could not create $config_dir"
    printf '{"device_id": "%s", "enabled": true, "api_base": "%s"}\n' "$device" "$api" >"$config_file" || fail "could not write $config_file"
    chmod 600 "$config_file" || fail "could not chmod $config_file"
    echo "wrote $config_file:"
    cat "$config_file"
  fi
fi

case ":${PATH:-}:" in
  *":$bindir:"*) ;;
  *) echo "add $bindir to your PATH to run bf from any shell" ;;
esac

"$bindir/bf" version
