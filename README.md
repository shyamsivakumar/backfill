# Backfill

**Get paid while your terminal waits.** A sponsored line rides the bottom row during `dbt run`, `cargo build`, `docker build`, and other long waits. Advertisers bid for the slot, you keep half. Open source, and it never reads your code.

[![release](https://img.shields.io/github/v/release/shyamsivakumar/backfill)](https://github.com/shyamsivakumar/backfill/releases) [![PyPI](https://img.shields.io/pypi/v/backfill-cli)](https://pypi.org/project/backfill-cli/) [![license](https://img.shields.io/github/license/shyamsivakumar/backfill)](LICENSE)

![bf on a real dbt run: per-model output collapses into one live line carrying the ad](assets/dbt.gif)

*A real `dbt run` under `bf`: the per-model `START`/`OK` noise collapses into one live line that carries the ad, with the header and `PASS/WARN/ERROR` summary intact.*

## Quickstart

```sh
pip install backfill-cli   # downloads + SHA-256-verifies the bf binary on first run
bf init                    # wrap dbt once, bare `dbt run` now earns
dbt run                    # runs exactly as before, with a footer that pays
```

`bf wrap cargo docker` adds more tools. `bf init --all` wraps everything slow on your `PATH`. `bf uninit` removes it cleanly. The explicit form `bf dbt run` always works with no setup.

## Install

Three install paths. All of them SHA-256 verify the downloaded binary.

```sh
# 1) PyPI (most common)
pip install backfill-cli

# 2) Homebrew
brew install shyamsivakumar/tap/backfill

# 3) curl installer
curl -fsSL https://backfill.sh/install.sh | sh
```

The pip wheel ships a thin Python entry point. On first run it fetches the matching `bf` release binary, verifies its SHA-256 against the published checksums, and execs it.

**macOS note.** Recent macOS (26 / "Tahoe") runs a Code Signing Monitor that silently kills an unsigned downloaded `bf` on launch, with no error to your shell. The pip and curl installers re-sign the binary ad-hoc (`codesign --sign -`) after download to clear this. If you copy a `bf` binary from somewhere else and hit it, run `codesign --force --sign - /path/to/bf` once and it works.

<details>
<summary><strong>Locked-down containers</strong> (Paradime, Codespaces, read-only base env)</summary>

When the base Python env isn't writable, `pip install` falls back to `--user` and puts `bf` in a dir that isn't on `PATH` (you'll see *"The script bf is installed in '.../.local/bin' which is not on PATH"*). That's a pip/platform thing, not a bf bug. Two ways through it:

```sh
# bootstrap without bf on PATH (self-heals PATH into your rc for next shell):
python -m backfill_cli init && exec $SHELL

# or use the explicit form, which never needs PATH:
python -m backfill_cli dbt run --select my_model
```

On **Paradime**, the durable path is `bf init`. It adds a real `export PATH="$HOME/.backfill/shims:$PATH"` to `~/.zshrc`, which the Code IDE terminal sources. Open a fresh terminal and `command -v dbt` should resolve to `~/.backfill/shims/dbt`. Avoid setting `PATH` through the Code IDE env-var UI: those values are stored literally, so a `${PATH}` reference won't expand and will clobber your shell's `PATH`.

`bf init` installs a pass-through shim per command into `~/.backfill/shims`. Because it's a real shim and not a shell alias, it fires wherever the command runs: your shell, a Makefile, a script.

</details>

## Commands

`bf` is a single Go binary (~600 lines, MIT). The full surface:

| Command | What it does |
|---|---|
| `bf <cmd>...` | Run `<cmd>` with the sponsored footer. No setup needed. |
| `bf init [cmd...]` | One-time setup. By default wraps **only `dbt`** by installing a PATH shim in `~/.backfill/shims/dbt` and prepending that dir to your `PATH` in your shell rc. Pass extra commands to wrap more. |
| `bf init --all` | Wrap every non-interactive command on your `PATH` (skips interactive tools: editors, shells, paginators, `sudo`, `ssh`, anything that takes over the screen). |
| `bf wrap <cmd>...` | Wrap the listed commands now (adds shims). |
| `bf unwrap <cmd>...` | Remove the shims for the listed commands. |
| `bf uninit` | Remove every shim `bf init` / `bf init --all` / `bf wrap` installed, and strip the `PATH` line from your rc. |
| `bf on` / `bf off` | Globally pause or resume the footer. `off` execs plainly with zero overhead, as if no shim is installed. |
| `bf status` | Show what's wrapped, current `on`/`off` state, and your device id and dashboard link. |
| `bf claim` | Print a one-time code and link to bind this device to your web account, so earnings show in your dashboard. |
| `bf refer` | Print your referral install command. |
| `bf statusline` | Print the sponsored status line that the agent integrations (Claude Code, Factory) call. |
| `bf agents install claude` | Install the Claude Code integration: ad replaces the thinking-spinner verb, plus an optional status line. |
| `bf agents install factory` | Install the Factory `droid` integration: ad in droid's status line. |
| `bf agents remove <name>` | Remove a previously installed agent integration. |
| `bf agents status` | Show which agent integrations are installed. |

`bf wrap droid` adds the live-spinner injection for Factory droid without the full agent install (handy for droid-specific sessions).

## How the wrapper works

`bf <cmd>` starts `<cmd>` in a PTY sized one row shorter than your terminal. It sets the DECSTBM scroll region to those rows, then draws one dim `ad …` line on the reserved bottom row. The line is an OSC 8 hyperlinked click target routed through a server-side redirect for click tracking.

- **stdin** is raw-mode passthrough, so Ctrl-C, line editing, and TUI input work normally.
- **SIGWINCH** is forwarded, and the PTY is resized for the child on every terminal resize.
- **Exit codes and child colors** are preserved end to end.
- **Non-TTY execs** (CI, Airflow, dbt Cloud, cron) run plainly with zero overhead: no PTY, no scroll-region trick. The shim detects this and just `exec`s the underlying binary.
- **Full-screen TUIs** (`vim`, `less`, `htop`, `top`, a pager) suppress the footer automatically via an alternate-screen guard: when the child enters the alternate screen buffer, `bf` yields the bottom row until it exits.

## Smart progress

For verbose commands, the per-line noise is the problem, not the wait. `bf` recognizes a set of command/verb pairs and switches to a collapsed live line instead of letting the footer fight a moving target.

| Command | Recognized verbs | What you see |
|---|---|---|
| `dbt` | `run`, `build`, `test`, `seed`, `snapshot` | One live line like `⠹ dbt 5/8 main.fct_orders · ad …`, the version header, any errors verbatim, and the final `PASS/WARN/ERROR` summary. |
| `sqlmesh` | `plan`, `run` | One live line carrying the model being applied and the ad. |

In both cases the ad rides the line your eyes are already on. The header stays, errors stay, the summary stays. Everything in between collapses.

## Scaffold completion ads

After any of the following exits 0, `bf` prints one persistent sponsored line under the success screen (one impression, regardless of how long the command took):

- `npm create` / `npm init`, `pnpm create` / `pnpm init`, `yarn create` / `yarn init`, `bun create` / `bun init`
- `npx create-*`
- `cargo new`, `cargo init`
- `django-admin startproject`
- `rails new`
- `dotnet new`
- any binary named `create-*` on your `PATH` that exits 0

The line prints once under whatever success UI the scaffolder drew. These commands finish too fast for the live footer to earn, but their "you're all set, here's what's next" screen is the highest-intent moment in the session.

## Coding agents

`bf` doesn't patch any agent's source. It uses each agent's own exposed surface.

| Agent | Integration | Install |
|---|---|---|
| Claude Code | Ad replaces the thinking-spinner verb, plus an optional status line | `bf agents install claude` |
| Factory (`droid`) | Status line + live-spinner injection | `bf agents install factory` (status line) + `bf wrap droid` (spinner) |
| Codex | Ad injected into the processing line via the spinner rewriter (no persistent install) | `bf spin codex` |
| Gemini CLI | No injectable status surface (full-screen TUI). Only the command-wait footer applies. | n/a |
| Cursor | No injectable status surface (full-screen TUI). Only the command-wait footer applies. | n/a |

For Claude Code you can also install via the plugin marketplace: `/plugin marketplace add shyamsivakumar/backfill` then `/plugin install backfill@backfill`. `bf agents remove claude` undoes it.

## What it sells that no other ad network can

- **Command-level segments.** Advertisers buy "developers currently running dbt," not "developers." The command name is the only targeting signal. No keywords, no profiles, no behavior graph.
- **Verified dwell.** A footer during a 15-minute compile is continuous, unskippable attention. There's no tab to switch away from without abandoning the build.
- **CI earnings routing.** Via the GitHub Action, a repo points its build-log earnings at its maintainers. Your CI minutes fund the dependencies you build on.

## Surfaces

| Surface | What's wrapped | How |
|---|---|---|
| dbt + data stack | `dbt`, `bq`, `snowsql`, `spark-submit`, `sqlmesh` | `bf init` (dbt only by default) or `bf wrap <cmd>` |
| Any CLI tool | `cargo`, `docker`, `make`, `terraform`, `gradle`, … | `bf wrap <cmd>` (or `bf init --all` for everything on `PATH`) |
| Coding agents | Claude Code spinner verb / status line, Factory droid status line + spinner | `bf agents install …` |
| Scaffold screens | `npm create`, `cargo new`, `rails new`, … | automatic on a clean wrapped run |
| CI build logs | GitHub Action in `action/` | maintainer-directed earnings |

## Privacy

The CLI is structurally incapable of reading your code, command args, command output, or environment. The only fields it ever transmits:

- **device id**, a random per-install id
- **ad id**, the campaign the server chose to serve
- **command name**, e.g. `dbt` or `cargo`, the bare basename, never the path
- **visible seconds**, how long the footer was actually on screen, rounded to whole seconds (1 impression = 5 visible seconds)
- **event kind**, a static label, `impression` or `click`

No args, no paths, no filenames, no env, no stdout or stderr contents. The source is open (MIT, ~600 lines of Go), so you can verify.

## Economics

- **Unit:** 1 impression = 5 visible seconds.
- **Pricing:** advertisers buy blocks of 1,000 impressions (CPM). Clicks bill at 50x an impression.
- **Split:** users keep 50% of attributable revenue (`USER_SHARE = 0.5`).
- **Balances** accrue per run and surface in `bf status`, `bf statusline`, and the web dashboard.
- **Payouts:** Stripe, once a balance crosses $25. Payout plumbing is planned, not live yet. Balances accrue today.
- **Early inventory:** while the first advertiser slots sell, the slot is filled with house ads at `cpm = 0`. No money changes hands, but the slot is exercised and you see a real sponsored line.

## Advertiser side

The web app is a Next.js (App Router) project on Drizzle + Neon Postgres.

### Routes

| Route | What |
|---|---|
| `/` | Landing page with the demo gif and the install one-liner |
| `/advertise` | Advertiser pitch page |
| `/advertiser` | Advertiser portal (auth-gated) |
| `/dashboard` | Per-device earnings dashboard (auth-gated) |
| `/login` | Magic-link auth (email only, no password) |
| `/stats` | Public stats page |
| `/dbt`, `/cargo`, `/terraform`, `/xcode`, `/claude-code`, `/codex`, `/ml-training`, `/vs` | Comparison / SEO pages |

### APIs

| Endpoint | Purpose |
|---|---|
| `GET /api/serve` | eCPM ad selection for a given device/command, frequency-capped and flight-gated |
| `POST /api/events` | Impression / click / visible-seconds events from `bf` |
| `POST /api/device/register` | Device registration / first-run handshake |
| `GET /api/balance` | Per-device balance for the dashboard |
| `GET, POST /api/postback` | Affiliate conversion postback |
| `POST /api/refer` | Referral attribution |
| `GET /api/stats` | Aggregate stats for the public stats page |
| `POST /api/admin/ads` | Admin actions (approve / toggle campaigns) |
| `POST /api/advertiser/account` | Advertiser account create/update |
| `POST /api/advertiser/campaign` | Campaign create/update (status `pending` until admin-approved) |
| `POST /api/advertiser/deposit` | Prepay Stripe deposit against the account |
| `POST /api/stripe/webhook` | Stripe checkout + webhook |
| `GET /api/cron/recompute-ecpm` | Nightly Bayesian-shrinkage eCPM recompute across campaigns |

### Ad selection

Every candidate gets a single unified **eCPM in micros**, then the server picks the max under frequency-cap and flight-window gating. The components:

- **Direct CPM**, what the advertiser pays per 1,000 impressions.
- **Affiliate expected value** = `payout × conversion-rate prior`, converted to an eCPM equivalent.
- **House floor**, the minimum to serve (currently 0 while house-ad inventory fills slots).

Bayesian shrinkage tempers noisy per-campaign conversion priors, so a campaign with 3 clicks doesn't outrank one with 3,000. The knobs in `web/lib/ecpm.ts`:

- `PRIOR_CVR_BPS = 50` (a 0.5% prior conversion rate)
- `PRIOR_STRENGTH = 500` (the prior weighted as 500 ghost conversions)

A frequency cap stops a device from seeing the same ad back to back across runs, and flight windows gate serving to a campaign's scheduled dates.

## Repo layout

| Dir | What |
|---|---|
| `cli/` | `bf`, the Go PTY wrapper (~600 lines). Owns the bottom row for one ad line. |
| `web/` | Next.js (App Router): landing, advertiser portal, dashboard, ad-serve + event API, Postgres via Drizzle on Neon. |
| `action/` | GitHub Action: the same model for CI build logs, with maintainer-directed earnings. |
| `python/` | Thin Python wrapper shipped in the `backfill-cli` wheel: fetches + SHA-256-verifies the Go binary, re-signs it ad-hoc on macOS, then execs it. |

## Tests

What's covered today:

- `cli/completion_test.go`, scaffold detection (the `create-*` / `cargo new` / `npm create` allowlist) and the one-line completion ad.
- `web/lib/ads.test.ts`, eCPM ad selection: frequency cap, flight gating, deterministic tie-break.
- `web/lib/ecpm.test.ts`, the eCPM math: direct CPM, affiliate expected value, Bayesian shrinkage with `PRIOR_CVR_BPS` and `PRIOR_STRENGTH`.
- `web/lib/advertiser.test.ts`, advertiser balance math: deposits, full-CPM spend, balance, rounding.

Run the web tests with `npm test` in `web/`. Run the CLI tests with `go test ./...` in `cli/`.

## License

MIT
