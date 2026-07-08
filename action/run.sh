#!/usr/bin/env bash
set +e

workflow_escape() {
  local value=${1-}
  value=${value//%/%25}
  value=${value//$'\r'/%0D}
  value=${value//$'\n'/%0A}
  printf '%s' "$value"
}

log_line_escape() {
  local value=${1-}
  value=${value//$'\r'/ }
  value=${value//$'\n'/ }
  printf '%s' "$value"
}

read_ad_json() {
  local parse_dir=$1

  AD_JSON=$AD_JSON python3 - "$parse_dir" <<'PY'
import json
import os
import pathlib
import sys

parse_dir = pathlib.Path(sys.argv[1])

try:
    payload = json.loads(os.environ.get("AD_JSON", ""))
except Exception:
    sys.exit(0)

if not isinstance(payload, dict):
    sys.exit(0)

ad_id = payload.get("id")
ad_text = payload.get("text")

if isinstance(ad_id, str):
    try:
        (parse_dir / "id").write_text(ad_id, encoding="utf-8")
    except OSError:
        pass

if isinstance(ad_text, str):
    try:
        (parse_dir / "text").write_text(ad_text, encoding="utf-8")
    except OSError:
        pass
PY
}

build_event_json() {
  python3 - "$BF_DEVICE" "$AD_ID" "$SECS" <<'PY'
import json
import sys

device_id, ad_id, seconds = sys.argv[1], sys.argv[2], int(sys.argv[3])
print(json.dumps({
    "deviceId": device_id,
    "adId": ad_id,
    "cmd": "ci",
    "seconds": seconds,
    "kind": "impression",
}, separators=(",", ":")))
PY
}

AD_JSON=$(curl -fsS --max-time 3 "$BF_API/api/serve?cmd=ci&d=$BF_DEVICE" || printf '')
AD_ID=
AD_TEXT=
PARSE_DIR=$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/backfill-action.XXXXXX" 2>/dev/null || printf '')

if [ -n "$PARSE_DIR" ]; then
  read_ad_json "$PARSE_DIR" || true
  [ -f "$PARSE_DIR/id" ] && AD_ID=$(cat "$PARSE_DIR/id")
  [ -f "$PARSE_DIR/text" ] && AD_TEXT=$(cat "$PARSE_DIR/text")
  rm -rf "$PARSE_DIR"
fi

if [ -n "$AD_TEXT" ]; then
  printf '::group::ad · %s\n' "$(workflow_escape "$AD_TEXT")"
  printf '%s\n' "$(log_line_escape "$BF_API/r/$AD_ID?d=$BF_DEVICE")"
  printf '::endgroup::\n'
fi

START=$(date +%s 2>/dev/null || printf '0')
bash -c "$BF_RUN"
EXIT=$?
END=$(date +%s 2>/dev/null || printf '%s' "$START")
SECS=$(( END - START ))
[ "$SECS" -lt 0 ] && SECS=0

if [ -n "$AD_ID" ] && [ -n "$BF_DEVICE" ] && [ "$SECS" -ge 5 ]; then
  EVENT_JSON=$(build_event_json || printf '')
  if [ -n "$EVENT_JSON" ]; then
    curl -fsS --max-time 3 -X POST "$BF_API/api/events" \
      -H 'content-type: application/json' \
      -d "$EVENT_JSON" \
      >/dev/null || true
  fi
fi

if [ -n "$AD_TEXT" ]; then
  printf 'ad · %s\n' "$(log_line_escape "$AD_TEXT")"
fi

exit "$EXIT"
