# re.balance

A savings-plan rebalancer. You own a handful of ETF positions with target weights (say 50/30/20).
Prices drift, so the actual weights no longer match. You invest a fixed amount every month.
`rebal` computes how to split that monthly amount across your positions so the portfolio converges
back to its targets over a chosen horizon — **without ever selling**.

No price fetching, no broker APIs, no selling or tax logic, no network access. It does one thing:
turn "here's what I hold and what I want" into "here's what to set each monthly savings plan to."

## Install

```sh
make install          # installs `rebal` into your GOBIN
# or
make build            # just compiles ./rebal in the repo root
```

Requires Go 1.26+.

## Quickstart

```sh
# 1. First run writes a starter config and tells you where. Edit it, or just
#    build your portfolio with the CLI (see below).
rebal check

# 2. Record what each position is currently worth (accepts 12450,00 / 12450.00 / "12 450 €").
rebal set IE00B4L5Y983 "12 450,00"
rebal set IE00BKM4GZ66 7310.50

# 3. See the plan: how many €/month to put into each position.
rebal plan
rebal plan --months 6 --monthly 500     # try a different horizon / amount

# 4. See how drift falls as the horizon grows, and when it converges.
rebal sweep
```

## Commands

`rebal` **owns** `config.toml` — the config-changing commands write it for you (atomically), so you
never hand-edit. Every writing command accepts `--dry-run` to preview the result without saving,
and refuses to write a config that fails validation.

| Command | What it does |
|---|---|
| `list` | Read-only view of positions, targets, values and drift. No solve; never fails. |
| `plan [--months N] [--monthly C]` | The contribution plan for one horizon. The daily driver. |
| `sweep [--max N]` | Horizon chart: max drift vs. months, with the convergence point. |
| `set <ISIN> <value>` | Record a position's current market value into state. |
| `check` | Validate config + state; non-zero exit on fatal issues. |
| `add --name … --isin … --target T [--scale \| --from …] [--min M]` | Add a position, make room for its target, write config. |
| `rm <ISIN> [--scale \| --to …]` | Remove a position, redistribute its target, write config. |
| `edit <ISIN> [--name …] [--min M \| --clear-min] [--target T [--scale \| --from …]]` | Change a position's name, floor or target. |
| `settings [--monthly N] [--tolerance X] [--horizon N] [--max-horizon N] [--default-min N]` | Show or change global settings. |
| `freeze <ISIN> [--scale \| --to …]` | Stop buying a position but keep holding it (wind-down). |
| `unfreeze <ISIN> <target> [--scale \| --from …]` | Resume buying a wind-down position. |

Global flags: `--config PATH`, `--state PATH`, `--no-color`.

Output is coloured when writing to a terminal (green/red drift by sign, dim zeros).
Colour turns off automatically when piped or redirected, or with `--no-color` / the
`NO_COLOR` environment variable.

### Making room for a target change

Any command that changes targets must keep them summing to 100, so it asks *where* the change comes
from or goes to:

- `--scale` — spread the change across the other positions in proportion to their current weight.
- `--from ISIN=pp` (taking) / `--to ISIN=pp` (giving) — explicit amounts, **repeatable**; the amounts
  must sum to the change. A bare `--from ISIN` (no `=`) takes the whole amount from that one position.

```sh
# Add a 10% position, taking 5pp from each of two others:
rebal add --name "World Small Cap" --isin IE00XXXXXXXX --target 10 \
    --from IE00B4L5Y983=5 --from IE00BKM4GZ66=5

# Raise one target to 55%, rescaling the rest to fit:
rebal edit IE00B4L5Y983 --target 55 --scale

# Stop buying a fund, handing its target back proportionally:
rebal freeze IE00B3VWMM18 --scale
```

## Config & state

Two files, deliberately separate:

- **`config.toml`** — settings and positions. Managed by `rebal`; a `# Managed by re.balance` header
  marks it. Located at `$XDG_CONFIG_HOME/rebal/config.toml` (or your OS config dir), or a
  `config.toml` in the current directory, or `--config PATH`.
- **`state.json`** — current market values (per ISIN, in cents), rewritten freely. At
  `$XDG_STATE_HOME/rebal/state.json` (or `~/.local/state/rebal/`), a local `state.json`, or
  `--state PATH`.

A new ISIN starts at 0 €; a deleted one becomes an ignored orphan — so adding and removing positions
is free, with no migration step. See [`config.example.toml`](config.example.toml) for the format.

## The target horizon (important)

There are two "horizon" numbers, and they mean different things:

- **Target horizon** — an **input you choose**: *"I want to be back on target within N months."* It's
  not just a readout — it **determines the monthly plan**. A *short* horizon front-loads corrections
  (aggressive catch-up into the underweight positions); a *long* horizon spreads them out (gentler,
  more even). So it's a real strategic choice: *how aggressively do I rebalance?*
- **Earliest on target** — a **computed result** shown under the chart (`earliest on target: month 4`):
  the *soonest* month at which you'd actually fall within tolerance.

They're related but distinct. Because drift falls as the horizon grows (more months = more total
contributed = fuller correction by the deadline), the earliest-on-target month is a threshold:

> **Pick a target horizon at or after the earliest-on-target month.** That month = "balanced as fast as
> possible"; longer = "I'm patient, give me a gentler split"; shorter = "won't make it in time."

In the TUI the drift readout next to the horizon (`Target horizon 6 months → ±1.20pp`) tells you exactly
where *your* choice lands, updating as you move it.

## How it works

The math is a separable-convex resource allocation: ignoring growth (contributions are linear), the
optimal integer split that minimises squared deviation from target end-values is "give each euro to
whoever is furthest below target." It's exact, deterministic, and costs ~a thousand comparisons — no
heap, no heuristics. See [`rebalance-spec.md`](rebalance-spec.md) for the full design (including the
wind-down rationale in §4).

### The optimisation, in full

For *n* positions with current values `v₁..vₙ`, target weights `w₁..wₙ` (fractions summing to 1), a
monthly budget `C`, a horizon of `M` months, and per-position monthly floors `m₁..mₙ`, choose monthly
allocations `c₁..cₙ` (`cᵢ ≥ mᵢ`, `Σcᵢ = C`) that bring the portfolio as close as possible to its targets
*M* months out — with no selling, so the only lever is where new money goes.

Ignoring growth, each position's end value is linear in its own allocation:

```
EndValueᵢ = vᵢ + M·cᵢ
T         = Σ EndValueᵢ = S + M·C        (S = current total value)
EndPctᵢ   = 100 · EndValueᵢ / T
```

"As close as possible" is defined as minimising the sum of squared weight deviations:

```
minimise  Σᵢ (EndPctᵢ − 100·wᵢ)²          subject to  cᵢ ≥ mᵢ,  Σcᵢ = C
```

Because `T` is fixed once `C` and `M` are chosen, minimising the weight deviation is the same as
minimising `Σᵢ (EndValueᵢ − wᵢ·T)²` — squared distance from each position's *target end value*, in
plain currency units. That reformulation is what makes the problem tractable.

### Why "give it to whoever's furthest behind" is exact, not a heuristic

Each position contributes `fᵢ(cᵢ) = (vᵢ + M·cᵢ − wᵢ·T)²` to the objective — a convex parabola in `cᵢ`.
Its slope is `f'ᵢ(cᵢ) = 2M·(vᵢ + M·cᵢ − wᵢ·T)`: the marginal benefit of the next euro to position *i*.
Every position shares the same `2M` factor, so **comparing marginal benefits across positions reduces
to comparing the raw term `vᵢ + M·cᵢ − wᵢ·T`** — exactly the "cost" the solver computes.

Minimising a sum of independent convex functions under one shared budget constraint is the textbook
*water-filling* problem: at the optimum, moving a single euro from position *i* to position *j* can never
lower the total. Handing each successive euro to the current `argmin` is precisely the rule that keeps
that condition true at every step — so the greedy result isn't an approximation of the optimal split,
it *is* the optimal split, and because the increments are whole currency units it's exactly reproducible
run to run (`internal/rebal/solve.go`; ties break to the lower index). No heap or LP solver is needed:
seed every position at its floor, then hand out the remaining budget one unit at a time, `rem` times —
a few hundred iterations for a typical monthly budget.

### Worked example

Three positions, no floors, a 1 000 €/month budget, current values 5 000 € / 2 000 € / 1 000 € against
50 % / 30 % / 20 % targets (`S` = 8 000 €). Compare a 1-month horizon against a 2-month one:

**Horizon = 1 month** — `T = 8 000 + 1·1 000 = 9 000 €`. Target end values: 4 500 € / 2 700 € / 1 800 €.
Only 1 000 € is available to close a combined 1 900 € shortfall on B and C, so the solver splits it
450 €/550 € between them (C is further behind, so it gets more) and A gets nothing:

| Position | Target | Monthly | End value | End % | Drift |
|---|---|---|---|---|---|
| A | 50 % | 0 € | 5 000 € | 55.56 % | **+5.56pp** |
| B | 30 % | 450 € | 2 450 € | 27.22 % | −2.78pp |
| C | 20 % | 550 € | 1 550 € | 17.22 % | −2.78pp |

Notice A drifts *over* target even though it received nothing — untouched money is a shrinking share of
a growing pie only when the pie grows enough, and here it doesn't: `T` grew by just 1 000 €, too little
to keep A's fixed 5 000 € down at 50 % of it.

**Horizon = 2 months** — `T = 8 000 + 2·1 000 = 10 000 €`. Now there's 2 000 € total to close the same
1 900 € shortfall, so it exactly covers it:

| Position | Target | Monthly | End value | End % | Drift |
|---|---|---|---|---|---|
| A | 50 % | 0 € | 5 000 € | 50.00 % | 0.00pp |
| B | 30 % | 500 € | 3 000 € | 30.00 % | 0.00pp |
| C | 20 % | 500 € | 2 000 € | 20.00 % | 0.00pp |

Zero drift — this horizon happens to be exactly long enough. That's the relationship `rebal sweep`
charts: drift shrinks as the horizon grows, and the "earliest on target" line marks the shortest horizon
where it hits zero (or, in general, falls within tolerance). See
[The target horizon](#the-target-horizon-important) above.

## TUI

Run `rebal` with no arguments to open the single-screen TUI: your positions, the live plan, the
monthly/horizon controls and the horizon chart, in one view. It's a thin skin over the same engine
and config the CLI uses — it edits the *what-if* knobs and re-solves instantly, while config
*structure* (adding positions, changing targets, wind-down) stays a CLI-only operation.

| Key | Action |
|---|---|
| `←` / `→` | move the target horizon (`shift` jumps 6, `home`/`end` to the ends) — re-solves live |
| `↑` / `↓`, `tab` / `shift+tab` | move focus between positions and the monthly field |
| `e` | edit the focused position's current value, or the monthly amount |
| `w` | save the current monthly + horizon as config defaults |
| `r` | reset monthly + horizon back to the saved defaults |
| `?` | toggle full help · `q` quit |

Current values **persist** to `state.json` (they're facts); the monthly amount and horizon are
transient what-ifs (use `w` to make them defaults). On quit, the plan you settled on prints to the
terminal — the €/position values ready to enter at your broker.

Config *structure* stays CLI-only by design, and the TUI never shells out to the CLI: both are
independent skins calling the same `internal/rebal` engine and `internal/config` directly.

## Status

Complete. The engine (`internal/rebal`) and chart renderer (`internal/ui/chart`) are pure
standard-library packages; the CLI and the TUI are both done. Mouse support (the one unbuilt spec
milestone) was deliberately skipped — the keyboard TUI is fully operable and mouse added little.

## Development

```sh
make test     # go test ./... -count=1
make vet
make fmt
make run ARGS="plan --months 6"
```
