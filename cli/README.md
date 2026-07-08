# bf

`bf` runs long commands so you can earn while builds, tests, migrations, and data jobs run. Wrapped non-interactive commands collapse into one live line that carries the ad, a trending repo / HN story / tip, and your $earned tally, with a spinner and an elapsed timer. Claude Code can get rotating spinner verbs while it thinks, Factory droid can get a statusLine via `bf agents install droid`, and Codex can run through `bf spin codex`. Revenue is split 50/50 with you. Privacy guarantee: `bf` never reads your code or command output; only the device id, ad id, command name, visible seconds, and event kind are transmitted.

## Install

```sh
brew tap shyamsivakumar/tap
brew install backfill
```

or:

```sh
curl -fsSL https://backfill.sh/install.sh | sh
```

## Usage

```sh
bf <cmd>     # wrap one command: bf dbt run
bf init      # one-time: make bare dbt/cargo/docker earn (installs PATH shims)
bf uninit    # remove the shims
bf on
bf off
bf status
bf claim
bf last
bf logs last
bf refer
bf agents install claude
bf agents install droid
bf wrap droid
bf spin codex
```

After `bf init`, plain `dbt run`, `dbt test`, `dbt build`, `cargo build`, and `docker build` earn without the `bf` prefix. The shim works wherever the command is launched (Makefiles, scripts, your shell). Interactive and full-screen commands (vim, less, ssh, sudo, gh, psql, `terraform apply`, `docker run -it`, `npm init` / `npm login`) are detected and run directly in your terminal, untouched. Non-interactive runs (CI, Airflow, dbt Cloud) detect no TTY and pass straight through with zero overhead.

<https://backfill.sh>
