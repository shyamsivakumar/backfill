# dbt Community Forum — "Show and Tell" — beta-tester post

**Category:** Show and Tell
**Title (primary):** built a thing that pays you while dbt runs, looking for a few testers
**Title alternates:**

- get paid for the time you spend watching dbt run? built this, need honest testers
- a terminal footer that earns while your models compile (looking for testers)

**Image placement:**

- Drag `assets/dbt.gif` in right under the first paragraph.
- Optionally drop `assets/screenshot-dbt-footer.png` near the bottom.

---

hey all. i'm shyam, i do data/analytics engineering. i spend a good chunk of my day watching dbt run, so i built a small thing for that dead time and i want a few people to tell me if it's actually useful or just annoying.

it's called backfill. you run `bf init` once, and after that your normal `dbt run` works exactly like before, except one sponsored line shows up in the bottom row while your models compile. you get paid for the wait. you keep 50%.

*(gif goes here)*

stuff i'd be suspicious of if i saw this, up front:

- it never reads your code, your sql, or your output. it wraps the command and owns one footer row, that's it.
- it's open source, one go binary: <https://github.com/shyamsivakumar/backfill>
- it's opt-in and reversible. `bf uninit` removes everything cleanly, nothing left behind.
- `bf init` only wraps dbt by default. it won't touch your other commands unless you run `bf wrap`.

and yeah, an ad in your terminal is a weird idea. i went back and forth on whether it's annoying. my guess is the wait is already dead time and a quiet line beats a spinner, but i genuinely don't know yet, which is why i'm asking here instead of just shipping it everywhere.

what i'm after:

- a few people who'll run it on real dbt work for a few days
- honest reactions. does the footer get in the way? would you actually leave it on? what makes you go "nope"?

to try it:

```
pip install backfill-cli
bf init
dbt run
```

sign in at <https://backfill.sh> to see what you've racked up.

on money, to be straight: you start accruing right away, but payouts only kick in once there's enough volume to be worth it (i'm not going to mail anyone 40 cents). i'll be clear about where that line is.

if you try it and hate it, telling me why helps just as much as telling me you like it. happy to get into how it works under the hood.

thanks
— shyam
