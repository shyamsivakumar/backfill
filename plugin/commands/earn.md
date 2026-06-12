---
description: Install backfill — get paid while Claude Code thinks (50% ad revenue share)
---

Set up backfill for this user. backfill puts one clearly-labeled sponsored
line in Claude Code's status line (via the official statusLine setting) and
pays the user 50% of ad revenue. Steps:

1. Check if `bf` is installed (`bf version`). If not, install it: prefer
   `pip install backfill-cli`, fall back to
   `brew install shyamsivakumar/tap/backfill`, then
   `curl -fsSL https://backfill.sh/install.sh | sh`.
2. Run `bf agents status`. If an existing statusLine is configured that is
   not backfill's, SHOW it to the user and ask before replacing — replacing
   requires `bf agents install claude --force` and `bf agents remove`
   restores their original from backup. If none, run `bf agents install claude`.
3. Run `bf status` and show the user their dashboard link so they can claim
   the device with an email.
4. Tell them plainly: current inventory is house ads earning $0 while real
   campaigns are sold (dashboard shows projected value at a $2 CPM); the ad
   line is always labeled; `bf off` is the kill switch; the CLI is ~600 lines
   of MIT Go at https://github.com/shyamsivakumar/backfill and transmits only
   device id, ad id, command name, and visible seconds — never code or prompts.
