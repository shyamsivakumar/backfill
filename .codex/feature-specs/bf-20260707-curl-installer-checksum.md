# LFD spec: Verify curl installer archives before execution

Idea ID: `bf-20260707-curl-installer-checksum`

Risk: medium

Status: unapproved

## Target

Make `install.sh` keep the README's install-security promise: the curl/wget installer must fetch the release `checksums.txt`, find the SHA-256 entry for the selected `backfill_<os>_<arch>.tar.gz` archive, verify the downloaded archive before extraction, and fail closed before executing any downloaded `bf` when verification cannot be completed.

Preserve the existing installer behavior outside verification:

- OS and architecture selection stays the same.
- curl and wget download paths both work.
- macOS ad-hoc resign still runs after extraction.
- `BACKFILL_API`, `BACKFILL_DEVICE`, and `BACKFILL_REF` behavior stays intact.
- The installer remains POSIX `sh`; no dependency install is allowed.

## Constraints

- Use only tools normally present on macOS and Linux install targets. Prefer `shasum -a 256` when available and `sha256sum` when available. If neither exists, fail with a clear error.
- Fetch `checksums.txt` from the same release download directory as the archive.
- Match the selected archive name exactly, accepting common checksum formats where the filename is either `name`, `*name`, or a path ending in `/name`.
- Treat missing checksum entries, conflicting entries, malformed digests, failed checksum downloads, and digest mismatches as hard failures.
- Keep temp files under the existing installer temp directory and clean them with the existing trap.
- Do not add calls to production Backfill services beyond the installer behavior that already exists.

## Instruments

- Add local shell-test coverage for the checksum parser and verifier. The tests may source helper functions from `install.sh` if the implementation splits them cleanly, or they may run the installer against local fixtures.
- Test at least: valid checksum, missing archive entry, malformed digest, duplicate conflicting digests, wrong digest, and both `sha256sum` and `shasum` command selection when practical.
- Run `sh -n install.sh`.
- Run `go test ./...` from `cli/` to confirm CLI changes were not affected.

## Forced entropy

The implementation must prove it fails before extraction/execution when verification is bad. A passing build should include a test or fixture that would execute a fake `bf` if checksum verification were skipped.

The manager will also run held-out fixture cases with decoy checksum rows, archive path prefixes, and a mutated tarball. Those answer keys must stay outside the worker spec.

## Acceptance checklist

- `install.sh` verifies the release archive SHA-256 before `tar -xzf`.
- Both curl and wget paths feed the same verification logic.
- Bad or unclear checksum state returns non-zero with a useful error.
- macOS resign and final `bf version` behavior are unchanged after a verified extraction.
- README claims require no wording downgrade.
