# How it works

The math is a separable-convex resource allocation: ignoring growth (contributions are linear), the
optimal integer split that minimises squared deviation from target end-values is "give each euro to
whoever is furthest below target." It's exact, deterministic, and costs ~a thousand comparisons — no
heap, no heuristics.

## The optimisation, in full

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

## Why "give it to whoever's furthest behind" is exact, not a heuristic

Each position contributes `fᵢ(cᵢ) = (vᵢ + M·cᵢ − wᵢ·T)²` to the objective — a convex parabola in `cᵢ`.
Its slope is `f'ᵢ(cᵢ) = 2M·(vᵢ + M·cᵢ − wᵢ·T)`: the marginal benefit of the next euro to position *i*.
Every position shares the same `2M` factor, so **comparing marginal benefits across positions reduces
to comparing the raw term `vᵢ + M·cᵢ − wᵢ·T`** — exactly the "cost" the solver computes.

Minimising a sum of independent convex functions under one shared budget constraint is the textbook
*water-filling* problem: at the optimum, moving a single euro from position *i* to position *j* can never
lower the total. Handing each successive euro to the current `argmin` is precisely the rule that keeps
that condition true at every step — so the greedy result isn't an approximation of the optimal split,
it *is* the optimal split, and because the increments are whole currency units it's exactly reproducible
run to run ([`internal/rebal/solve.go`](../internal/rebal/solve.go); ties break to the lower index). No heap or LP solver is needed:
seed every position at its floor, then hand out the remaining budget one unit at a time, `rem` times —
a few hundred iterations for a typical monthly budget.

## Worked example

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
[The target horizon](usage.md#the-target-horizon-important) above.

## Wind-down positions

A frozen position (`wind_down = true`) is one you keep holding but no longer buy. Because `rebal`
never sells, its weight can only decay toward zero without ever reaching it, so it can't carry a
target or count toward drift. Otherwise it would read as "never on target" forever. Instead it
leaves the target pool entirely:

- Active targets sum to 100 among themselves: freezing the 20 in a 50/30/20 split gives 62.5/37.5.
- Drift is measured over active positions only, so the plan still converges.
- The frozen value still counts toward the displayed portfolio total.

It's an explicit flag rather than `target = 0` on purpose: TOML reads a missing value as `0`, so a
forgotten target would silently mean "stop buying". A test pins the behaviour down: freezing a
position must give the same active plan as deleting it.
