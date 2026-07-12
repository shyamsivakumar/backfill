# LFD spec: Smoke test plugin auto-activation hook

Idea ID: `bf-20260712-plugin-activation-smoke`

Risk: medium

Status: unapproved

## Target

Add a local smoke test for the Claude plugin auto-activation hook.

`plugin/hooks/hooks.json` runs `sh "${CLAUDE_PLUGIN_ROOT}/hooks/activate.sh"` on
SessionStart. The hook backgrounds discovery of an existing `bf`, falls back to
`$HOME/.local/bin/bf` and `$HOME/.local/share/backfill/bf`, optionally runs the
curl installer, and finally invokes `bf spinner-refresh`. There is no local
fixture covering that shell behavior, so plugin packaging can regress without
Go tests noticing.

## Constraints

- Tests must not call the network.
- Tests must not execute the real installer.
- Tests must not depend on Claude Code being installed.
- Tests must verify the hook exits promptly and does not block on background
  work.
- Keep shell portability: `activate.sh` is `/bin/sh`, not bash.
- Do not change plugin consent semantics or install behavior unless a test
  exposes a real local bug.

## Instruments

- Add a shell smoke test, likely under `tests/`, that runs
  `plugin/hooks/activate.sh` with temporary `HOME`, `PATH`, and fake commands.
- Use fake `bf` executables to record whether `spinner-refresh` was invoked.
- Use a fake `curl` command for installer-fallback coverage, proving no network
  call is made during the test.
- Validate `plugin/hooks/hooks.json` still points at the packaged hook path.
- Run `sh -n plugin/hooks/activate.sh` and the new smoke test.

## Forced entropy

The fixtures must include:

- A `bf` found on `PATH`.
- A `bf` found only at `$HOME/.local/bin/bf`.
- A missing `bf` path where fake `curl` simulates installer success.
- A missing `bf` path where fake `curl` fails and the hook still exits zero.
- A fake `bf` that records argv so the test proves `spinner-refresh` is the
  command invoked.

The manager will keep held-out cases for spaced plugin roots, non-executable
fallback files, empty `PATH`, installer output suppression, and backgrounded
work completing after the hook process exits.

## Acceptance checklist

- The hook passes `sh -n`.
- The hook invokes `bf spinner-refresh` when `bf` is discoverable.
- The installer fallback is tested with a fake `curl`, not the network.
- The hook exits zero when `bf` cannot be installed.
- `hooks.json` still references `hooks/activate.sh`.
