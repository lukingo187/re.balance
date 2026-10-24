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
# 1. First run writes a starter config and tells you where.
rebal check

# 2. Record what each position is currently worth (accepts 12450,00 / 12450.00 / "12 450 €").
rebal set IE00B4L5Y983 "12 450,00"
rebal set IE00BKM4GZ66 7310.50

# 3. See the plan: how many €/month to put into each position.
rebal plan
rebal plan --months 6 --monthly 500     # try a different horizon / amount

# 4. See how drift falls as the horizon grows, and when it converges.
rebal sweep

# Or run `rebal` with no arguments for the interactive TUI.
```

## Docs

| | |
|---|---|
| [Usage](docs/usage.md) | All commands, changing targets, config & state, the target horizon, TUI keys |
| [How it works](docs/how-it-works.md) | The optimisation, why the greedy split is exact, a worked example |
| [rebal-sc](docs/rebal-sc.md) | Optional addon: sync values from and push plans to Scalable Capital via `sc` |
| [Design record](docs/rebalance-spec.md) | Why `rebal` is built the way it is |
| [Scalable integration](docs/scalable-integration.md) | Why `rebal-sc` is built the way it is, safety rules |

## Status

Complete: engine, CLI and TUI. `rebal-sc` is an optional addon and never a dependency of `rebal`.

## Development

```sh
make test     # go test ./... -count=1
make vet
make fmt
make run ARGS="plan --months 6"
```
