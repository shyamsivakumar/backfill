#!/bin/sh
set -eu

script_dir="$(
	unset CDPATH
	cd -- "$(dirname -- "$0")" && pwd
)"
root_dir="$(
	unset CDPATH
	cd -- "$script_dir/.." && pwd
)"

_BACKFILL_INSTALL_TESTING=1 . "$root_dir/install.sh"

test_tmp="${TMPDIR:-/tmp}/backfill-install-tests.$$"
mkdir -p "$test_tmp"
trap 'rm -rf "$test_tmp"' EXIT HUP INT TERM

archive_name="backfill_linux_amd64.tar.gz"
zero_digest="0000000000000000000000000000000000000000000000000000000000000000"
one_digest="1111111111111111111111111111111111111111111111111111111111111111"

fail_test() {
	echo "FAIL: $*" >&2
	exit 1
}

real_sha256() {
	file="$1"

	if command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$file" | awk '{ print $1; exit }'
	elif command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$file" | awk '{ print $1; exit }'
	else
		fail_test "shasum or sha256sum is required to run checksum tests"
	fi
}

write_archive_bytes() {
	file="$1"
	printf 'archive fixture: %s\n' "$file" >"$file"
}

expect_failure() {
	if "$@" >/dev/null 2>"$test_tmp/last-error.log"; then
		fail_test "expected failure: $*"
	fi
}

test_valid_checksum_with_path_prefix() {
	archive="$test_tmp/valid.tar.gz"
	checksums="$test_tmp/valid-checksums.txt"
	write_archive_bytes "$archive"
	digest="$(real_sha256 "$archive")"

	{
		printf '%s  %s\n' "$zero_digest" "decoy-${archive_name}"
		printf '%s  %s\n' "$digest" "release/${archive_name}"
	} >"$checksums"

	verify_archive_checksum "$archive" "$checksums" "$archive_name"
}

test_valid_checksum_with_star_prefix() {
	archive="$test_tmp/star.tar.gz"
	checksums="$test_tmp/star-checksums.txt"
	write_archive_bytes "$archive"
	digest="$(real_sha256 "$archive")"

	printf '%s *%s\n' "$digest" "$archive_name" >"$checksums"

	verify_archive_checksum "$archive" "$checksums" "$archive_name"
}

test_missing_archive_entry_fails() {
	archive="$test_tmp/missing.tar.gz"
	checksums="$test_tmp/missing-checksums.txt"
	write_archive_bytes "$archive"

	printf '%s  %s\n' "$zero_digest" "other.tar.gz" >"$checksums"

	expect_failure verify_archive_checksum "$archive" "$checksums" "$archive_name"
}

test_malformed_digest_fails() {
	archive="$test_tmp/malformed.tar.gz"
	checksums="$test_tmp/malformed-checksums.txt"
	write_archive_bytes "$archive"

	printf '%s  %s\n' "not-a-sha256" "$archive_name" >"$checksums"

	expect_failure verify_archive_checksum "$archive" "$checksums" "$archive_name"
}

test_conflicting_duplicate_digest_fails() {
	archive="$test_tmp/conflict.tar.gz"
	checksums="$test_tmp/conflict-checksums.txt"
	write_archive_bytes "$archive"

	{
		printf '%s  %s\n' "$zero_digest" "$archive_name"
		printf '%s  %s\n' "$one_digest" "dist/${archive_name}"
	} >"$checksums"

	expect_failure verify_archive_checksum "$archive" "$checksums" "$archive_name"
}

test_wrong_digest_fails() {
	archive="$test_tmp/wrong.tar.gz"
	checksums="$test_tmp/wrong-checksums.txt"
	write_archive_bytes "$archive"

	printf '%s  %s\n' "$zero_digest" "$archive_name" >"$checksums"

	expect_failure verify_archive_checksum "$archive" "$checksums" "$archive_name"
}

test_checksum_tool_prefers_shasum() (
	tool_dir="$test_tmp/tools-with-shasum"
	mkdir -p "$tool_dir"
	printf '#!/bin/sh\nexit 0\n' >"$tool_dir/shasum"
	printf '#!/bin/sh\nexit 0\n' >"$tool_dir/sha256sum"
	chmod +x "$tool_dir/shasum" "$tool_dir/sha256sum"

	PATH="$tool_dir"
	tool="$(checksum_tool)" || return 1
	[ "$tool" = "shasum" ]
)

test_checksum_tool_uses_sha256sum_without_shasum() (
	tool_dir="$test_tmp/tools-with-sha256sum"
	mkdir -p "$tool_dir"
	printf '#!/bin/sh\nexit 0\n' >"$tool_dir/sha256sum"
	chmod +x "$tool_dir/sha256sum"

	PATH="$tool_dir"
	tool="$(checksum_tool)" || return 1
	[ "$tool" = "sha256sum" ]
)

test_bad_checksum_stops_before_extracting_fake_bf() {
	work_dir="$test_tmp/entropy"
	payload_dir="$work_dir/payload"
	extract_dir="$work_dir/extract"
	archive="$work_dir/$archive_name"
	checksums="$work_dir/checksums.txt"
	marker="$work_dir/fake-bf-ran"

	mkdir -p "$payload_dir" "$extract_dir"
	cat >"$payload_dir/bf" <<EOF
#!/bin/sh
printf ran >"$marker"
exit 0
EOF
	chmod +x "$payload_dir/bf"

	(cd "$payload_dir" && tar -czf "$archive" bf)
	printf '%s  %s\n' "$zero_digest" "$archive_name" >"$checksums"

	expect_failure extract_verified_archive "$archive" "$checksums" "$archive_name" "$extract_dir" "linux"
	[ ! -e "$marker" ] || fail_test "fake bf ran despite checksum failure"
	[ ! -e "$extract_dir/bf" ] || fail_test "archive extracted despite checksum failure"
}

run_test() {
	name="$1"
	shift

	"$@" || fail_test "$name"
	printf 'ok - %s\n' "$name"
}

run_test "valid checksum with path prefix" test_valid_checksum_with_path_prefix
run_test "valid checksum with star prefix" test_valid_checksum_with_star_prefix
run_test "missing archive entry fails" test_missing_archive_entry_fails
run_test "malformed digest fails" test_malformed_digest_fails
run_test "conflicting duplicate digest fails" test_conflicting_duplicate_digest_fails
run_test "wrong digest fails" test_wrong_digest_fails
run_test "checksum tool prefers shasum" test_checksum_tool_prefers_shasum
run_test "checksum tool uses sha256sum without shasum" test_checksum_tool_uses_sha256sum_without_shasum
run_test "bad checksum stops before extracting fake bf" test_bad_checksum_stops_before_extracting_fake_bf
