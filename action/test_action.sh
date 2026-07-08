#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
TMP_ROOT=$(mktemp -d "${TMPDIR:-/tmp}/backfill-action-tests.XXXXXX")
trap 'rm -rf "$TMP_ROOT"' EXIT

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

write_fake_tools() {
  local bin_dir=$1

  mkdir -p "$bin_dir"

  cat >"$bin_dir/curl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

is_post=0
data=

while [ "$#" -gt 0 ]; do
  case "$1" in
    -X)
      [ "${2-}" = "POST" ] && is_post=1
      shift 2
      ;;
    -d)
      data=${2-}
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [ "$is_post" -eq 1 ]; then
  printf '%s\n' "$data" >>"$FAKE_CURL_EVENTS"
  [ "${FAKE_CURL_POST_FAIL:-0}" = "1" ] && exit 22
  exit 0
fi

[ "${FAKE_CURL_SERVE_FAIL:-0}" = "1" ] && exit 22
cat "$FAKE_SERVE_FILE"
SH

  cat >"$bin_dir/date" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

if [ "${1-}" != "+%s" ]; then
  /bin/date "$@"
  exit $?
fi

count=0
if [ -f "$FAKE_DATE_COUNT" ]; then
  count=$(cat "$FAKE_DATE_COUNT")
fi
count=${count:-0}

if [ "$count" -eq 0 ]; then
  printf '%s\n' "${FAKE_DATE_START:-100}"
else
  printf '%s\n' "${FAKE_DATE_END:-100}"
fi

printf '%s\n' "$((count + 1))" >"$FAKE_DATE_COUNT"
SH

  chmod +x "$bin_dir/curl" "$bin_dir/date"
}

write_json() {
  local path=$1
  local ad_id=$2
  local ad_text=$3

  python3 - "$path" "$ad_id" "$ad_text" <<'PY'
import json
import pathlib
import sys

path, ad_id, ad_text = sys.argv[1], sys.argv[2], sys.argv[3]
pathlib.Path(path).write_text(json.dumps({"id": ad_id, "text": ad_text}), encoding="utf-8")
PY
}

run_action() {
  local name=$1
  local device=$2
  local command=$3
  local serve_file=$4
  local post_fail=${5:-0}
  local end_time=${6:-100}
  local serve_fail=${7:-0}
  local case_dir="$TMP_ROOT/$name"

  mkdir -p "$case_dir"
  write_fake_tools "$case_dir/bin"
  : >"$case_dir/events"
  : >"$case_dir/date-count"

  set +e
  BF_API="https://example.test" \
    BF_DEVICE="$device" \
    BF_RUN="$command" \
    GITHUB_ACTION_PATH="$ROOT/action" \
    FAKE_SERVE_FILE="$serve_file" \
    FAKE_CURL_EVENTS="$case_dir/events" \
    FAKE_CURL_POST_FAIL="$post_fail" \
    FAKE_CURL_SERVE_FAIL="$serve_fail" \
    FAKE_DATE_COUNT="$case_dir/date-count" \
    FAKE_DATE_START=100 \
    FAKE_DATE_END="$end_time" \
    PATH="$case_dir/bin:$PATH" \
    bash "$ROOT/action/run.sh" >"$case_dir/stdout" 2>"$case_dir/stderr"
  status=$?
  set -e

  printf '%s\n' "$status" >"$case_dir/status"
}

assert_status() {
  local name=$1
  local want=$2
  local got
  got=$(cat "$TMP_ROOT/$name/status")
  [ "$got" = "$want" ] || fail "$name: expected status $want, got $got"
}

assert_contains() {
  local file=$1
  local needle=$2
  grep -Fq "$needle" "$file" || fail "$file: missing [$needle]"
}

assert_not_line_start() {
  local file=$1
  local needle=$2
  ! grep -q "^$needle" "$file" || fail "$file: found forbidden line [$needle]"
}

ad_text=$'quoted "value" %\r\n::error::forged annotation\nnext'
write_json "$TMP_ROOT/adversarial.json" 'ad"42' "$ad_text"
run_action adversarial device-1 'printf "wrapped ok\n"' "$TMP_ROOT/adversarial.json" 1 106
assert_status adversarial 0
assert_contains "$TMP_ROOT/adversarial/stdout" '::group::ad · quoted "value" %25%0D%0A::error::forged annotation%0Anext'
assert_contains "$TMP_ROOT/adversarial/stdout" 'wrapped ok'
assert_contains "$TMP_ROOT/adversarial/stdout" 'ad · quoted "value" %  ::error::forged annotation next'
assert_not_line_start "$TMP_ROOT/adversarial/stdout" '::error::'
assert_contains "$TMP_ROOT/adversarial/events" '"adId":"ad\"42"'

lone_command_text=$'::error::starts as a command'
write_json "$TMP_ROOT/lone-command.json" ad-123 "$lone_command_text"
run_action lone_command device-1 'exit 0' "$TMP_ROOT/lone-command.json"
assert_status lone_command 0
assert_contains "$TMP_ROOT/lone_command/stdout" 'ad · ::error::starts as a command'
assert_not_line_start "$TMP_ROOT/lone_command/stdout" '::error::starts as a command'

write_json "$TMP_ROOT/id-newline.json" $'ad\n123' 'plain ad'
run_action id_newline device-1 'exit 0' "$TMP_ROOT/id-newline.json" 0 106
assert_status id_newline 0
assert_contains "$TMP_ROOT/id_newline/stdout" 'https://example.test/r/ad 123?d=device-1'
assert_contains "$TMP_ROOT/id_newline/events" '"adId":"ad\n123"'

printf '{' >"$TMP_ROOT/malformed.json"
run_action malformed device-1 'exit 0' "$TMP_ROOT/malformed.json"
assert_status malformed 0
[ ! -s "$TMP_ROOT/malformed/events" ] || fail "malformed: impression should not post"

printf '{}' >"$TMP_ROOT/empty.json"
run_action empty device-1 'exit 0' "$TMP_ROOT/empty.json"
assert_status empty 0
[ ! -s "$TMP_ROOT/empty/events" ] || fail "empty: impression should not post"

printf '{"text":"text without id"}' >"$TMP_ROOT/missing-id.json"
run_action missing_id device-1 'exit 0' "$TMP_ROOT/missing-id.json" 0 106
assert_status missing_id 0
assert_contains "$TMP_ROOT/missing_id/stdout" 'ad · text without id'
[ ! -s "$TMP_ROOT/missing_id/events" ] || fail "missing_id: impression should not post"

write_json "$TMP_ROOT/missing-device.json" ad-123 'plain ad'
run_action missing_device '' 'exit 0' "$TMP_ROOT/missing-device.json" 0 106
assert_status missing_device 0
[ ! -s "$TMP_ROOT/missing_device/events" ] || fail "missing_device: impression should not post"

write_json "$TMP_ROOT/under-threshold.json" ad-123 'plain ad'
run_action under_threshold device-1 'exit 0' "$TMP_ROOT/under-threshold.json" 0 104
assert_status under_threshold 0
[ ! -s "$TMP_ROOT/under_threshold/events" ] || fail "under_threshold: impression should not post"

run_action failed_fetch device-1 'exit 0' "$TMP_ROOT/empty.json" 0 100 1
assert_status failed_fetch 0
[ ! -s "$TMP_ROOT/failed_fetch/events" ] || fail "failed_fetch: impression should not post"

write_json "$TMP_ROOT/nonzero.json" ad-123 'plain ad'
run_action nonzero device-1 'exit 7' "$TMP_ROOT/nonzero.json"
assert_status nonzero 7

write_json "$TMP_ROOT/quoted-command.json" ad-123 'plain ad'
run_action quoted_command device-1 'printf "quoted value\n"; exit 3' "$TMP_ROOT/quoted-command.json"
assert_status quoted_command 3
assert_contains "$TMP_ROOT/quoted_command/stdout" 'quoted value'

printf 'ok\n'
