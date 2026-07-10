# LFD spec: Document GitHub Action setup and failure behavior

Idea ID: `bf-20260707-ci-action-docs`

Risk: low

Status: unapproved

## Target

Add copy-paste documentation for using the Backfill GitHub Action in CI.

The README currently mentions CI earnings routing and identifies
`action/action.yml`, but it does not show a complete workflow or explain how
device crediting, command failure, missing device ids, privacy, and permission
scope behave.

## Constraints

- Documentation-only unless an obvious action metadata typo is found.
- Do not change action runtime behavior.
- Do not recommend permissions broader than the action needs.
- Do not imply Backfill can read command output, repo contents, secrets, or pull
  request metadata.
- Keep examples usable for public repositories and forks.

## Instruments

- Add a README section with a minimal GitHub Actions workflow using
  `uses: shyamsivakumar/backfill/action@<tag-or-sha>` and the `run` and
  `device` inputs.
- Explain how to find a device id from `bf status` and where maintainers should
  store it.
- Explain that the wrapped command exit code passes through.
- Explain that missing `device` can still render an ad line but does not credit
  an impression.
- Explain privacy fields consistently with the existing Privacy section.
- Add a short local check, such as `bash action/test_action.sh`, to the
  acceptance notes if behavior examples mention tested action paths.

## Forced entropy

The docs must address:

- A normal `push` workflow.
- A pull request workflow from a fork where secrets may be unavailable.
- A non-zero wrapped command.
- A missing device id.
- A custom API base for self-test or staging use without recommending it for
  normal users.

The manager will keep held-out review questions about whether the example leaks
secrets to forks, whether the action needs write permissions, and whether action
logs can forge workflow commands.

## Acceptance checklist

- README has a copy-paste CI example.
- Device crediting and missing-device behavior are documented.
- Failure pass-through is documented.
- Privacy claims are consistent with the existing Privacy section.
- Any referenced behavior is backed by existing action tests or a new docs-only
  note that does not overclaim.
