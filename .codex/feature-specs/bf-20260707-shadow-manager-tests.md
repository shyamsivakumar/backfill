# LFD spec: Add shell rc shadow-manager tests

Idea ID: `bf-20260707-shadow-manager-tests`

Risk: low

Status: unapproved

## Target

Add Go tests around Backfill's shell rc setup and version-manager warning logic.

`bf init` appends the Backfill shim PATH block and warns when a version manager appears after that block, because the manager can re-prepend PATH and hide the shim. This behavior should be covered without touching the developer's real shell files.

## Constraints

- Tests must use temp files and temp directories only.
- Do not modify `~/.zshrc`, `~/.bashrc`, `~/.backfill`, or the real PATH.
- Cover `detectShadowingManagers`, `stripBlock`, legacy marker removal, and idempotent block replacement where practical.
- If `writeRCBlocks` needs a small seam for testing, keep the default production path behavior unchanged.
- Keep command output wording stable unless a test exposes a real issue.

## Instruments

- Add Go tests in `cli/` for manager markers after the Backfill block, manager markers before the block, multiple managers, missing or partial markers, current block stripping, and legacy block stripping.
- Include at least `nvm`, `fnm`, `mise`, `pyenv`, and one non-manager false positive case.
- Run `go test ./...` from `cli/`.

## Forced entropy

The tests must include rc files with:

- Backfill block followed by more than one manager.
- Manager setup before the Backfill block, which should not warn.
- Uppercase or mixed-case manager text.
- A partial Backfill marker pair that should not corrupt unrelated rc content.
- Both current and legacy Backfill blocks in one file.

The manager will keep held-out rc snippets with comments, duplicate blocks, and manager names inside unrelated prose.

## Acceptance checklist

- Shadow-manager detection has direct unit coverage.
- Backfill rc block stripping has direct unit coverage.
- Tests do not touch real user files.
- `go test ./...` passes in `cli/`.
