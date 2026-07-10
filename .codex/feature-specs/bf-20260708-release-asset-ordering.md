# LFD spec: Gate PyPI publishing on release assets

Idea ID: `bf-20260708-release-asset-ordering`

Risk: high

Status: unapproved

## Target

Prevent the PyPI wheel from publishing before the matching GitHub release
archives and `checksums.txt` are available.

The Python launcher fetches the `bf` archive for the installed package version
from the GitHub release and verifies it against `checksums.txt`. Both
`.github/workflows/release.yml` and `.github/workflows/publish-pypi.yml`
currently trigger independently on `v*` tags, so PyPI can become visible before
the binary assets needed by first-run installs exist.

## Constraints

- High risk: this changes release automation. It requires explicit
  `approved_for_high_risk_build` before implementation.
- Do not publish, push tags, create releases, or call production package
  registries during implementation.
- Do not touch PyPI trusted-publishing secrets or repository secrets.
- Preserve the existing tag-based release UX unless the spec approval explicitly
  chooses a new release flow.
- Keep rollback simple: a failed release should leave no partial PyPI publish
  if release assets are absent.

## Instruments

- Convert PyPI publishing to depend on successful release asset production, or
  add a deterministic pre-publish gate that verifies the release contains all
  expected archives plus `checksums.txt`.
- Add local validation for the workflow structure, using static checks or a
  small script where practical.
- Document the intended release order in the workflow or release notes.
- If a gate script is added, test it against fixture JSON for complete,
  incomplete, and mismatched releases without hitting GitHub.

## Forced entropy

The tests or static fixtures must include:

- A complete release with linux, darwin, and expected architecture archives plus
  `checksums.txt`.
- A release missing `checksums.txt`.
- A release missing one archive.
- A release where `checksums.txt` exists but does not mention an expected
  archive.
- A prerelease or draft response that should not unlock PyPI publishing.

The manager will keep held-out fixture cases with extra assets, changed asset
ordering, duplicate asset names, and tag names with a leading `v`.

## Acceptance checklist

- PyPI publish cannot run before matching binary release assets are present.
- The release gate is testable without network access.
- Existing release permissions are not broadened.
- The rollback path for a failed gate is clear.
- High-risk approval is recorded before any build starts.
