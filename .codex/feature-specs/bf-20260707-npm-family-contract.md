# LFD spec: Clarify npm-family wrapped run contract

Idea ID: `bf-20260707-npm-family-contract`

Risk: low

Status: unapproved

## Target

Make the package-manager wrapping contract internally consistent across code,
tests, and README copy.

Current evidence points in two directions:

- `cli/completion.go` says npm, pnpm, yarn, and bun invocations always run
  plainly with no footer.
- `cli/run.go` only routes install-family commands through the plain path, then
  emits a completion ad on success.
- `README.md` says package installs run through the collapsed live line while
  resolving and downloading.

The build should choose the product behavior deliberately and make comments,
docs, and tests agree.

## Constraints

- Do not change non-package command routing.
- Do not add network calls or shell out to real npm, pnpm, yarn, or bun in
  tests.
- Keep CI and non-TTY behavior plain.
- Preserve scaffold completion ads for `npm create`, `npm init`, and peer
  commands unless the chosen contract explicitly changes them.
- If behavior changes, keep user-visible copy accurate about earnings,
  completion ads, and failure behavior.

## Instruments

- Add or update Go tests around package-manager classification for npm, pnpm,
  yarn, bun, and near misses such as npx and create-* binaries.
- Add a focused test or small helper that proves `runWrapped` selects the
  intended path for installs, run scripts, scaffolds, and ordinary commands
  without executing real package managers.
- Update comments in `cli/completion.go` and `cli/run.go` to match the chosen
  contract.
- Update the README npm/package-install section so it no longer contradicts the
  implementation.
- Run `go test ./...` from `cli/`.

## Forced entropy

The tests must include:

- `npm install`, `npm i`, `npm ci`, and `npm update`.
- `pnpm add`, `yarn` with no subcommand, and `bun install`.
- `npm run build`, `npm test`, `pnpm dev`, and `bun run start`.
- Scaffold commands such as `npm create`, `pnpm init`, and `npx create-app`.
- Near misses such as `npminstall`, `bunx`, and `create-npm`.

The manager will keep held-out cases with flags before subcommands, workspace
flags after subcommands, empty arg lists, and package managers invoked through
shim names.

## Acceptance checklist

- The chosen npm-family behavior is explicit in tests.
- README behavior claims match the implementation.
- Comments no longer claim unused or contradictory behavior.
- Scaffold completion ads still have coverage.
- `go test ./...` passes in `cli/`.
