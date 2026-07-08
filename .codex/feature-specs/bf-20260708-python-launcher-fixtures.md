# LFD spec: Add Python launcher checksum and cache fixtures

Idea ID: `bf-20260708-python-launcher-fixtures`

Risk: medium

Status: unapproved

## Target

Add dependency-free Python test coverage for the `backfill-cli` launcher in `python/backfill_cli/__init__.py`.

The tests must cover the launcher behavior that protects users during `pip install backfill-cli` and `pip install -U backfill-cli`: checksum lookup, archive verification, version-keyed cache paths, old binary pruning, shell PATH self-healing, and failure before executing an unverified binary.

The preferred outcome is a stdlib `unittest` suite under `python/` that can run without installing extra packages.

## Constraints

- Do not add Python package dependencies.
- Do not call GitHub, Backfill production services, `codesign`, or a real downloaded binary.
- Keep tests isolated from the developer's real home directory, shell rc files, PATH, and cached `~/.local/share/backfill` directory.
- Use local fixtures, monkeypatching, temp directories, or small fake archives to exercise `_read_expected_sha256`, `_download`, `_prune_old_binaries`, `_shell_export_path_line`, `_ensure_on_path`, and `main`-level path choice where practical.
- If production code needs small seams to make tests clean, keep them narrow and preserve the public `bf = backfill_cli:main` entry point.
- Do not change release URLs, checksum rules, cache directory names, or macOS resign behavior unless a test exposes a real defect and the fix stays in scope.

## Instruments

- Add tests runnable with `python3 -m unittest discover python`.
- Run `python3 -m py_compile python/backfill_cli/__init__.py`.
- Run `python3 -m unittest discover python`.
- Run `go test ./...` from `cli/` to check the Go CLI was not affected.

## Forced entropy

The suite must include adversarial fixture cases that would pass if a test only checked the happy path:

- `checksums.txt` has decoy rows for other archives.
- A matching row uses a path prefix or `*archive` form.
- Duplicate matching rows disagree.
- The archive bytes are mutated after the expected digest is chosen.
- The release archive contains no regular-file `bf`.
- A PATH candidate contains spaces or shell-significant characters.

The manager will keep held-out cases for malformed checksum rows, duplicate equivalent rows, newline-bearing paths, and a fake `bf` that must not run when verification fails.

## Acceptance checklist

- Python launcher tests run without network access.
- Checksum parsing and mismatch failures are covered.
- Archive extraction cannot execute or install an unverified `bf`.
- Version-keyed cache and old-binary pruning behavior are covered.
- PATH self-healing writes only to temp rc files during tests.
