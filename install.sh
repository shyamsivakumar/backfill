#!/bin/sh
set -eu

repo="shyamsivakumar/backfill"

err() {
	echo "error: $*" >&2
}

fail() {
	err "$*"
	exit 1
}

download_file() {
	download_url="$1"
	dest="$2"

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$download_url" -o "$dest"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$dest" "$download_url"
	else
		return 127
	fi
}

checksum_tool() {
	if command -v shasum >/dev/null 2>&1; then
		printf '%s\n' "shasum"
	elif command -v sha256sum >/dev/null 2>&1; then
		printf '%s\n' "sha256sum"
	else
		return 1
	fi
}

compute_sha256() {
	file="$1"
	tool="$(checksum_tool)" || {
		err "shasum or sha256sum is required to verify downloads"
		return 1
	}

	case "$tool" in
	shasum) shasum -a 256 "$file" | awk '{ print $1; exit }' ;;
	sha256sum) sha256sum "$file" | awk '{ print $1; exit }' ;;
	*) return 1 ;;
	esac
}

is_sha256_hex() {
	printf '%s\n' "$1" | awk '
    length($0) == 64 && $0 ~ /^[[:xdigit:]]+$/ { ok = 1 }
    END { exit ok ? 0 : 1 }
  '
}

find_checksum_digest() {
	checksum_file="$1"
	archive_name="$2"

	awk -v want="$archive_name" '
    function is_hex_digest(value) {
      return length(value) == 64 && value ~ /^[[:xdigit:]]+$/
    }

    function checksum_name_matches(value) {
      if (substr(value, 1, 1) == "*") {
        value = substr(value, 2)
      }

      if (value == want) {
        return 1
      }

      return length(value) > length(want) &&
        substr(value, length(value) - length(want), 1) == "/" &&
        substr(value, length(value) - length(want) + 1) == want
    }

    checksum_name_matches($2) {
      if (!is_hex_digest($1)) {
        print "error: malformed checksum digest for " want > "/dev/stderr"
        failed = 1
        exit 1
      }

      digest = tolower($1)
      if (!seen) {
        expected = digest
        seen = 1
      } else if (expected != digest) {
        print "error: conflicting checksum entries for " want > "/dev/stderr"
        failed = 1
        exit 1
      }
    }

    END {
      if (failed) {
        exit 1
      }
      if (!seen) {
        print "error: no checksum entry found for " want > "/dev/stderr"
        exit 1
      }
      print expected
    }
  ' "$checksum_file"
}

verify_archive_checksum() {
	archive="$1"
	checksum_file="$2"
	archive_name="$3"

	expected="$(find_checksum_digest "$checksum_file" "$archive_name")" || return 1
	actual="$(compute_sha256 "$archive")" || return 1
	actual="$(printf '%s\n' "$actual" | tr 'A-F' 'a-f')"

	if ! is_sha256_hex "$actual"; then
		err "could not compute SHA-256 for $archive_name"
		return 1
	fi

	if [ "$actual" != "$expected" ]; then
		err "checksum mismatch for $archive_name"
		return 1
	fi
}

extract_verified_archive() {
	archive="$1"
	checksum_file="$2"
	archive_name="$3"
	extract_dir="$4"
	install_os="$5"

	verify_archive_checksum "$archive" "$checksum_file" "$archive_name" || return 1
	tar -xzf "$archive" -C "$extract_dir" || {
		err "could not extract archive"
		return 1
	}

	[ -x "$extract_dir/bf" ] || {
		err "archive did not contain executable bf"
		return 1
	}

	# macOS 15.4+/26's Code Signing Monitor SIGKILLs the binary's build-machine
	# ad-hoc signature on exec. Re-sign ad-hoc locally so it runs here.
	if [ "$install_os" = "darwin" ] && command -v codesign >/dev/null 2>&1; then
		codesign --force --sign - "$extract_dir/bf" >/dev/null 2>&1 || true
	fi

	"$extract_dir/bf" version >/dev/null 2>&1 || {
		err "downloaded bf did not run successfully"
		return 1
	}
}

install_main() {
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
	checksums_url="${url%/*}/checksums.txt"

	tmp="${TMPDIR:-/tmp}/backfill-install.$$"
	mkdir -p "$tmp"
	trap 'rm -rf "$tmp"' EXIT HUP INT TERM

	echo "downloading $url"

	archive="$tmp/$name"
	checksums="$tmp/checksums.txt"
	download_file "$url" "$archive" || {
		rc=$?
		[ "$rc" -eq 127 ] && fail "curl or wget is required"
		fail "download failed"
	}

	download_file "$checksums_url" "$checksums" || {
		rc=$?
		[ "$rc" -eq 127 ] && fail "curl or wget is required"
		fail "checksum download failed"
	}

	extract_verified_archive "$archive" "$checksums" "$name" "$tmp" "$os" || exit 1

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

	if [ "${BACKFILL_REF+x}" ]; then
		ref="${BACKFILL_REF-}"
		case "$ref" in
		*\"* | *\\*)
			echo "warning: BACKFILL_REF contains a double quote or backslash; not sending referral" >&2
			;;
		*)
			api_base="${BACKFILL_API-https://backfill.sh}"
			device_id="$("$bindir/bf" status 2>/dev/null | awk '/^device:/ { print $2; exit }')"
			if [ -n "$device_id" ] && command -v curl >/dev/null 2>&1; then
				curl -fsS -X POST "$api_base/api/refer" \
					-H "Content-Type: application/json" \
					-d "$(printf '{"refereeDeviceId":"%s","referrerDeviceId":"%s"}' "$device_id" "$ref")" >/dev/null 2>&1 || true
			fi
			;;
		esac
	fi

	case ":${PATH:-}:" in
	*":$bindir:"*) ;;
	*) echo "add $bindir to your PATH to run bf from any shell" ;;
	esac

	"$bindir/bf" version
}

if [ "${_BACKFILL_INSTALL_TESTING:-}" != "1" ]; then
	install_main
fi
