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
bf <cmd>
bf init
bf uninit
bf on
bf off
bf status
```

## How the PTY footer works

`bf` runs your command inside a pseudo-terminal so interactive programs keep behaving normally. It reserves a small footer area at the bottom of the terminal and redraws sponsored content there while forwarding your input and the command output. The wrapped command still receives the same terminal signals and exits with its original status.

https://backfill.sh
