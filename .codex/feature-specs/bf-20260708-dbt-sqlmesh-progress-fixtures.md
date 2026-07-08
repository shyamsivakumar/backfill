# LFD spec: Add dbt and SQLMesh progress parser fixtures

Idea ID: `bf-20260708-dbt-sqlmesh-progress-fixtures`

Risk: low

Status: unapproved

## Target

Add focused Go tests for Backfill's dbt and SQLMesh smart-progress parsers.

The tests must prove that routine tool noise is collapsed, meaningful headers and summaries stay visible, errors and warnings pass through, model counts advance correctly, and command-family detection picks only the intended dbt and SQLMesh subcommands.

No test should require installing or running dbt or SQLMesh.

## Constraints

- Use checked-in fixture strings and unit tests around parser functions, renderer handlers, or small extracted pure helpers.
- Do not call the Backfill API. Use fake configs, fake rotator inputs, or narrower helper tests where needed.
- Do not make terminal-dependent tests rely on a real TTY.
- Keep receipt behavior unchanged except for defects directly exposed by the new fixtures.
- Keep production refactors small. The goal is coverage of current parser contracts, not a rewrite of rendering.

## Instruments

- Add Go tests for `isDbtRunFamily`, `isSqlmeshRunFamily`, dbt model/progress extraction, SQLMesh step/backfill extraction, and `isDbtNoise` / `isSqlmeshNoise`.
- Include fixture lines with ANSI color codes, progress glyphs, dbt PASS/WARN/FAIL text, SQLMesh Rich progress output, Python warning chatter, and summary lines.
- Run `go test ./...` from `cli/`.

## Forced entropy

The fixtures must include near-miss lines that should not be counted as model progress:

- A dbt line with numbers but no supported state.
- A SQLMesh elapsed-time or progress-bar line that contains `%`.
- A warning line that should be suppressed only when it is routine Python warning noise.
- An error line that must pass through even when it resembles normal progress chatter.

The manager will keep held-out dbt and SQLMesh output snippets with ANSI resets split across lines, unknown warning text, and model names that include dots and underscores.

## Acceptance checklist

- dbt and SQLMesh parser tests are checked in.
- Error and warning visibility is protected by tests.
- Routine noise suppression is protected by tests.
- Command-family detection ignores flags before the subcommand.
- `go test ./...` passes in `cli/`.
