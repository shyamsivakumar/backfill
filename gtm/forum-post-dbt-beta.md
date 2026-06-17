# dbt Community Forum — "Show and Tell" — beta-tester post

**Category:** Show and Tell
**Title (primary):** I built a CLI that pays you while dbt runs, and I want a few beta testers
**Title alternates:**

- Get paid for the time you spend watching dbt run? I built this and need honest testers
- A footer that earns while your models compile (looking for beta testers)

**Image placement:**

- Drag `assets/dbt-real.gif` in right under the first paragraph (the hero).
- Optionally drop `assets/screenshot-dbt-footer.png` near the bottom.

---

Hey all. I'm Shyam, I work in data and analytics engineering, and I've spent a real fraction of my life watching `dbt run` grind through models. So I built something for that dead time, and I'd like a few of you to tell me whether it's actually useful or just annoying.

It's called backfill. It's a small CLI. You run `bf init` once, and after that your normal `dbt run` behaves exactly like before, except a single sponsored line shows up in the bottom row of your terminal while your models compile. You get paid for that wait time. You keep 50%.

*(gif goes here)*

Here are the things I'd be skeptical about if I saw this, answered up front:

- It never reads your code, your SQL, your model output, or your results. It wraps the command and owns one footer row. That's the whole surface area.
- It's open source. One Go binary, you can read exactly what it does.
- It's opt-in and reversible. `bf uninit` removes the shims and the PATH line cleanly, nothing left behind.
- `bf init` only wraps `dbt` by default. It won't touch your other commands unless you ask it to with `bf wrap`.

And yes, an ad in your terminal is a strange idea. I went back and forth on whether it's obnoxious. My bet is that the wait is already dead time, the line stays quiet and out of the way, and getting paid for it beats staring at a spinner. But I honestly don't know yet, which is why I'm posting here instead of on a billboard.

What I'm looking for:

- A handful of people willing to run it on real dbt work for a few days.
- Brutally honest reactions. Does the footer get in the way? Does it feel intrusive? Would you actually leave it on? Anything that makes you go "nope."

How to try it:

```
pip install backfill-cli
bf init
dbt run
```

Sign in at <https://backfill.sh> to see what you've accrued.

One honest note on money: you accrue earnings from day one, but actual payouts kick in once there's enough volume to make them worth doing (I'm not going to mail anyone 40 cents). I'll be upfront about where that threshold lands.

If you try it and hate it, telling me why is just as useful as telling me you like it. Happy to answer anything about how it works under the hood.

Thanks for reading.

— Shyam
