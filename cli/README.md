# bf

`bf` runs long commands with a sponsored terminal footer so you can earn while builds, tests, migrations, and data jobs run. Revenue is split 50/50 with you. Privacy guarantee: `bf` never reads your code or command output; only the ad id, command name, and visible seconds are transmitted.

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
```

After `bf init`, plain `dbt run`, `dbt test`, `dbt build`, `cargo build`, and
`docker build` earn without the `bf` prefix. The shim works wherever the command
is launched (Makefiles, scripts, your shell). Non-interactive runs (CI, Airflow,
dbt Cloud) detect no TTY and pass straight through with no footer.

## How the PTY footer works

`bf` runs your command inside a pseudo-terminal so interactive programs keep behaving normally. It reserves a small footer area at the bottom of the terminal and redraws sponsored content there while forwarding your input and the command output. The wrapped command still receives the same terminal signals and exits with its original status.

<https://backfill.sh>
