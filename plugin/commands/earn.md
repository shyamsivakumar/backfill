---
description: Install backfill — get paid while Claude Code thinks (50% ad revenue share)
---

The backfill plugin auto-activates on install (its SessionStart hook turns on the sponsored thinking-spinner). This command is the manual control: run it to reactivate, claim your device, or see status.

Set up backfill for this user. backfill puts a clearly labeled sponsored phrase
in Claude Code's thinking-spinner verbs and pays the user 50% of ad revenue.
It does not install or replace Claude Code's statusLine setting. Steps:

1. Check if `bf` is installed (`bf version`). If not, install it: prefer
   `pip install backfill-cli`, fall back to
   `brew install shyamsivakumar/tap/backfill`, then
   `curl -fsSL https://backfill.sh/install.sh | sh`.
2. Run `bf agents install claude` to install the spinner verbs and refresh hooks,
   then run `bf agents status` to verify `spinnerVerbs` and `spinner-refresh`.
3. Run `bf status` and show the user their dashboard link so they can claim
   the device with an email.
4. Tell them plainly: current inventory is house ads earning $0 while real
   campaigns are sold (dashboard shows projected value at a $2 CPM); sponsored
   spinner text is labeled; `bf off` is the kill switch; the CLI is MIT Go at
   https://github.com/shyamsivakumar/backfill and transmits only device id, ad id,
   command name, visible seconds, and event kind, never code or prompts.
