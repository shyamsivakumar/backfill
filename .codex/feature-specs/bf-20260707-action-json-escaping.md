# LFD spec: Harden GitHub Action ad JSON and log escaping

Idea ID: `bf-20260707-action-json-escaping`

Risk: medium

Status: unapproved

## Target

Make `action/action.yml` safe around ad-server data. The composite action must parse the `/api/serve` response as JSON, escape or sanitize values before they touch GitHub workflow command syntax, preserve the wrapped command's exit code, and keep impression posting best-effort.

The specific weak points are local to the action:

- `AD_JSON` is parsed with `sed`, which can misread escaped JSON.
- `AD_TEXT` is written into `::group::...`, where `%`, CR, and LF have GitHub workflow-command meaning.
- Server-provided text is printed to the CI log without stripping line breaks, so a malicious or malformed ad payload can forge extra log commands.

## Constraints

- Keep the action composite and dependency-free for normal GitHub-hosted runners. `bash`, `curl`, and `python3` are acceptable.
- Do not change the public inputs: `run`, `device`, and `api`.
- Do not change successful command execution semantics. The user's `run` input still controls the command and the final action exit code must equal that command's exit code.
- Do not print secrets or environment dumps. `device` may appear only where it already appears in the ad click URL or event payload.
- Failed ad fetches, malformed ad JSON, missing ad IDs, and failed impression posts must not fail the user's CI command.

## Instruments

- Add local tests or a script fixture that exercises the action run logic with fake `/api/serve` responses. It may extract the shell block into a checked-in script if that makes testing simpler.
- Test at least: escaped JSON quotes, ad text containing `%`, CR, LF, and a `::error::`-style substring; malformed JSON; empty ad; missing device; wrapped command exit code 0 and non-zero.
- Run syntax checks for any new shell files.
- Run `go test ./...` from `cli/` if shared code is touched.

## Forced entropy

The build should include an adversarial ad text case that would create a fake GitHub annotation if printed raw. The fixed action must show it as inert text and still run the requested command.

The manager will keep a held-out fake API response corpus with multiline JSON strings, command-looking text, and missing fields. The answer keys must stay outside the worker spec.

## Acceptance checklist

- JSON parsing uses a real parser instead of `sed` string extraction.
- Values in workflow-command positions are escaped for GitHub command syntax.
- Plain log lines strip or replace CR/LF from server-provided text.
- The action exits with the wrapped command's exit code.
- Ad fetch and impression failures stay best-effort.

