# Backfill

**Get paid while your terminal waits.** A sponsored line rides the bottom row during `dbt run`, `cargo build`, `docker build`, and other long waits. Advertisers bid for the slot, you keep half. Open source, and it never reads your code.

[![release](https://img.shields.io/github/v/release/shyamsivakumar/backfill)](https://github.com/shyamsivakumar/backfill/releases) [![PyPI](https://img.shields.io/pypi/v/backfill-cli)](https://pypi.org/project/backfill-cli/) [![license](https://img.shields.io/github/license/shyamsivakumar/backfill)](LICENSE)

![bf on a real dbt run — the footer holds the bottom row while models build](assets/dbt-real.gif)

*A real `dbt run` under `bf`: the output scrolls, the sponsored line stays pinned to the bottom row the whole run, summary and exit code intact.*

## Quickstart

```sh
pip install backfill-cli   # downloads + SHA-256-verifies the bf binary on first run
bf init                    # wrap dbt once — bare `dbt run` now earns
dbt run                    # runs exactly as before, with a footer that pays
```

`bf wrap cargo docker` adds more tools. `bf init --all` wraps everything slow on your `PATH`. `bf uninit` removes it cleanly. The explicit form `bf dbt run` always works with no setup.

## What it sells that no other ad network can

- **Command-level segments** — advertisers buy "developers currently running dbt," not "developers." The command name is the only targeting signal.
- **Verified dwell** — a footer during a 15-minute compile is continuous, unskippable attention. There's no tab to switch away from without abandoning the build.
- **CI earnings routing** — via the GitHub Action, a repo points its build-log earnings at its maintainers. Your CI minutes fund the dependencies you build on.

## It can't read your code

The CLI is ~600 lines of Go you can read in ten minutes, and it's structurally incapable of reading your code, command output, or environment. The only fields it ever transmits: device id, ad id, command name (e.g. `dbt`), and visible seconds.

## How it works

`bf <cmd>` starts `<cmd>` in a PTY one row shorter than your terminal, pins the scroll region to those rows (DECSTBM), and draws one dim `ad …` line on the reserved bottom row, hyperlinked via OSC 8 through a click-tracking redirect. stdin is raw-mode passthrough, SIGWINCH resizes both, exit codes are preserved. With no TTY (CI, Airflow, dbt Cloud) or `bf off`, it execs plainly with zero overhead. Full-screen TUIs (`vim`, `less`, `htop`) suppress the footer automatically via the alternate-screen guard.

For `dbt run`/`build`/`test`/`seed`/`snapshot`, `bf` switches to **smart progress**: it collapses the hundreds of per-node `START`/`OK` lines into one live line (`⠹ dbt 5/8 main.fct_orders · ad …`), keeps the version header, errors, and the final `PASS/WARN/ERROR` summary, and carries the ad on the line you're watching. Scaffold commands (`npm create`, `cargo new`) print one sponsored line under their success screen.

## Coding agents

```sh
bf agents install claude     # ad replaces the thinking-spinner verb + status line
bf agents install factory    # ad in droid's status line
bf wrap droid                # ad on droid's live spinner
```

- **Claude Code** — ad in the thinking-spinner verb, plus an optional status line.
- **Factory (`droid`)** — status line and live-spinner injection.
- Install via the Claude Code plugin: `/plugin marketplace add shyamsivakumar/backfill` then `/plugin install backfill@backfill`.

## Economics

- 1 impression = 5 visible seconds. Advertisers buy blocks of 1,000 (CPM). Clicks bill at 50×.
- You keep 50% of attributable revenue. Balances accrue per run; Stripe payouts once you cross $25.

## Repo layout

| Dir | What |
|---|---|
| `cli/` | `bf`, the Go PTY wrapper. Runs any command, owns the bottom row for one ad line. |
| `web/` | Next.js: landing, advertiser portal, dashboard, ad-serve + event API, Postgres via Drizzle. |
| `action/` | GitHub Action: the same model for CI build logs. |

<details>
<summary><strong>Locked-down containers</strong> (Paradime, Codespaces, read-only base env)</summary>

When the base Python env isn't writable, `pip install` falls back to `--user` and puts `bf` in a dir that isn't on `PATH` (you'll see *"The script bf is installed in '.../.local/bin' which is not on PATH"*). That's a pip/platform thing, not a bf bug. Two ways through it:

```sh
# bootstrap without bf on PATH (self-heals PATH into your rc for next shell):
python -m backfill_cli init && exec $SHELL

# or use the explicit form, which never needs PATH:
python -m backfill_cli dbt run --select my_model
```

On **Paradime**, the durable path is `bf init` — it adds a real `export PATH="$HOME/.backfill/shims:$PATH"` to `~/.zshrc`, which the Code IDE terminal sources. Open a fresh terminal and `command -v dbt` should resolve to `~/.backfill/shims/dbt`. Avoid setting `PATH` through the Code IDE env-var UI: those values are stored literally, so a `${PATH}` reference won't expand and will clobber your shell's `PATH`.

`bf init` installs a pass-through shim per command into `~/.backfill/shims`. Because it's a real shim and not a shell alias, it fires wherever the command runs: your shell, a Makefile, a script.

</details>

## License

MIT
