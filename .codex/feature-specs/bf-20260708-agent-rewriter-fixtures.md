# LFD spec: Add Codex and Factory spinner rewrite fixtures

Idea ID: `bf-20260708-agent-rewriter-fixtures`

Risk: low

Status: unapproved

## Target

Add fixture-driven tests for the Codex and Factory spinner rewriter used by
`bf spin` and wrapped agent commands.

`spinnerRewriter.transform` rewrites PTY frames when stable interrupt anchors
are present. It replaces known spinner verbs with the sponsored label, or
injects the label before the anchor when the verb is unknown. Existing tests
cover agent argument parsing and Codex config cleanup, but not the actual frame
rewrite contract.

## Constraints

- Tests must not launch Codex, Factory, Claude, or any terminal agent.
- Tests must not require a real TTY.
- Keep fixture data local and small; avoid capturing private prompts, paths, or
  command output.
- Do not change billing or impression reporting behavior unless a test exposes
  a real bug.
- Preserve ordinary output pass-through when no supported spinner anchor exists.

## Instruments

- Add Go unit tests for `spinnerRewriter.transform` and `spinnerAdBytes`.
- Cover Codex anchors with `Esc to interrupt` and Factory anchors with
  `Press ESC to stop`, including lowercase variants.
- Cover known verbs (`Working`, `Executing...`, `Executing...` with ellipsis
  variants, and `Streaming...`) and the fallback path for unknown gerund verbs.
- Cover idempotency enough to show repeated frames do not stack multiple ads in
  the same frame.
- Cover ordinary prose or logs containing words such as "Working" without an
  interrupt anchor.
- Run `go test ./...` from `cli/`.

## Forced entropy

The fixtures must include:

- ANSI color/control sequences around the spinner verb.
- A frame split with carriage return and partial status text.
- An unknown verb before a valid anchor.
- A prose line that mentions "Esc to interrupt" outside a spinner-like frame.
- A long ad text that exercises spinner label capping.

The manager will keep held-out frames with mixed-case anchors, UTF-8 ellipses,
multiple anchors in one PTY chunk, and multiline chunks containing both normal
logs and a spinner frame.

## Acceptance checklist

- Codex spinner frames rewrite to a single sponsored label.
- Factory spinner frames rewrite to a single sponsored label.
- Fallback injection works for unknown verbs.
- Non-spinner output passes through unchanged.
- Long ad labels are capped by test.
- `go test ./...` passes in `cli/`.
