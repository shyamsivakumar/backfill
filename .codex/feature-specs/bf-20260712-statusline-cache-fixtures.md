# LFD spec: Add statusline cache and refresh fixtures

Idea ID: `bf-20260712-statusline-cache-fixtures`

Risk: low

Status: unapproved

## Target

Add focused tests for the statusline cache and refresh behavior used by
`bf statusline`, `bf statusline-refresh`, and agent status line integrations.

`cli/statusline.go` reads a cached ad, strips control characters before output,
prints an OSC 8 hyperlink line, refreshes stale cache entries in the background,
posts bounded impressions during refresh, and uses a lock file to avoid stacked
refreshes. Current tests cover agent config parsing, spinner rewriting, and ad
payload helpers, but there is no statusline-specific test file.

## Constraints

- Tests must not call the real Backfill API.
- Tests must not launch Claude, Codex, Factory, or any GUI process.
- Tests must isolate `HOME` with a temporary directory.
- Avoid sleeping on wall-clock time except for stale-lock timestamp setup via
  file modtime.
- Keep production behavior unchanged unless the tests expose a small local bug.

## Instruments

- Add Go unit tests under `cli/`, likely `statusline_test.go`.
- Cover valid cache read/write and invalid cache rejection.
- Cover control-character stripping for cached `Ad` fields before printing.
- Cover `printStatuslineAd` output shape, including the escaped ad id and device
  id inside the hyperlink target.
- Cover refresh-lock behavior for new, fresh, and stale lock files.
- Run `go test ./...` from `cli/`.

## Forced entropy

The fixtures must include:

- A cached ad whose id, text, URL, and spinner text contain control characters.
- An ad id that needs URL path escaping.
- A device id that needs query escaping.
- A malformed cache JSON file.
- A fresh lock file and a stale lock file with modtimes controlled by the test.

The manager will keep held-out cases for empty ad ids, empty ad text, zero
`fetched_at`, mixed path/query escaping, and stale locks whose recreate attempt
fails.

## Acceptance checklist

- Cache read rejects malformed or incomplete cache files.
- Cache read sanitizes all cached ad fields.
- Printed statusline output preserves text while emitting a safe hyperlink.
- Refresh lock acquisition is single-owner for fresh locks and recovers stale
  locks.
- `go test ./...` passes in `cli/`.
