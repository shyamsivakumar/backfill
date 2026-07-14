# LFD: Centralize and test ad click link encoding

Idea ID: `bf-20260714-ad-link-encoding-fixtures`
Risk: medium

## Target

Backfill should build ad click tracker links through one tested helper so every OSC 8 renderer encodes ad IDs and device IDs consistently. The target surfaces are `completionAdLine`, `footer.draw`, `collapseRenderer.link`, `adRotator.link`, and `statuslineAdLine`.

## Constraints

- Do not change the public ad API shape, click tracker route shape, command-line flags, or emitted non-link copy.
- Do not call production systems, add network tests, or require a live ad server.
- Preserve the special `earnings` behavior where the earnings entry links directly to `Ad.URL` and is never billed as a click.
- Keep control-character stripping for server-supplied IDs before building links.
- Prefer a small shared helper over repeating `fmt.Sprintf("%s/r/%s?d=%s", ...)` variants.

## Instruments

- Add Go unit coverage for the shared click-link helper with reserved characters in both `Ad.ID` and `Config.DeviceID`.
- Add or extend renderer-level tests so completion, footer, collapsed progress, rotator, and statusline all use the same encoded `/r/<ad>?d=<device>` form.
- Include at least one assertion that `earnings` links continue to bypass the click tracker.
- Run `gofmt` on changed Go files.
- Run `env GOCACHE=/private/tmp/backfill-gocache go test ./...` from `cli/`.
- Run `git diff --check`.

## Forced entropy

The implementation worker should not receive a single golden answer string as the only acceptance key. Use at least three adversarial fixture values that include a slash, space, query delimiter, ampersand, percent sign, and a stripped control character across ad ID and device ID inputs. Keep the expected encoded outputs inside the tests, not in the worker prompt. A valid build must pass the tests rather than merely matching a manually listed string.

## Acceptance checklist

- [ ] A shared helper constructs click tracker hrefs with `url.PathEscape` for the ad ID and `url.QueryEscape` for the device ID.
- [ ] Existing renderers use the helper or an equivalent single tested path.
- [ ] Statusline behavior stays compatible with the current escaped-link behavior.
- [ ] `earnings` entries still link directly to the underlying URL.
- [ ] Local Go tests and diff checks pass.
