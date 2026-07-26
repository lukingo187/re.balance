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
- **`state.json`** — current market values (per ISIN, in cents), rewritten freely, with a per-day
  history. At `$XDG_STATE_HOME/rebal/state.json` (or `~/.local/state/rebal/`), a local `state.json`,
  or `--state PATH`.

A new ISIN starts at 0 €; a deleted one becomes an ignored orphan — so adding and removing positions
is free, with no migration step. See [`config.example.toml`](config.example.toml) for the format.

## How it works

The math is a separable-convex resource allocation: ignoring growth (contributions are linear), the
optimal integer split that minimises squared deviation from target end-values is "give each euro to
whoever is furthest below target." It's exact, deterministic, and costs ~a thousand comparisons — no
heap, no heuristics. **Convergence horizon** is just the first month where max drift falls within
your tolerance. See [`rebalance-spec.md`](rebalance-spec.md) for the full design, and
[`docs/wind-down-positions.md`](docs/wind-down-positions.md) for the wind-down rationale.

## Status

The CLI is complete and is the whole product; the engine (`internal/rebal`) and chart renderer
(`internal/ui/chart`) are pure standard-library packages. A single-screen TUI — where the horizon
chart becomes a live cursor that re-solves the plan as you move it — is the planned next milestone,
built as a thin skin over the same engine and config-mutation layer the CLI already uses.

## Development

```sh
make test     # go test ./... -count=1
make vet
make fmt
make run ARGS="plan --months 6"
```
