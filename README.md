# re.balance

[![CI](https://github.com/lukingo187/re.balance/actions/workflows/ci.yml/badge.svg)](https://github.com/lukingo187/re.balance/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/lukingo187/re.balance)](https://github.com/lukingo187/re.balance/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

A savings-plan rebalancer. You own a handful of ETF positions with target weights (say 50/30/20).
Prices drift, so the actual weights no longer match. You invest a fixed amount every month.
`rebal` computes how to split that monthly amount across your positions so the portfolio converges
back to its targets over a chosen horizon — **without ever selling**.

No price fetching, no broker APIs, no selling or tax logic, no network access. It does one thing:
turn "here's what I hold and what I want" into "here's what to set each monthly savings plan to."

## Install

```sh
brew install lukingo187/tap/rebal     # macOS / Linux, installs rebal and rebal-sc
```

Or download an archive from [Releases](https://github.com/lukingo187/re.balance/releases), or build
from source (Go 1.26+):

```sh
go install github.com/lukingo187/re.balance/cmd/rebal@latest
# or, from a checkout
make install
```

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

## Scalable Capital integration

Investing with Scalable Capital? The optional `rebal-sc` addon uses Scalable's official
[`sc` CLI](https://github.com/ScalableCapital/scalable-cli) to pull current values from your account
and turn `rebal`'s plan into savings-plan changes, always as a preview first, with a separate
confirm step. See [rebal-sc](docs/rebal-sc.md).

## Docs

| | |
|---|---|
| [Usage](docs/usage.md) | All commands, changing targets, config & state, the target horizon, TUI keys |
| [How it works](docs/how-it-works.md) | The optimisation, why the greedy split is exact, a worked example |
| [rebal-sc](docs/rebal-sc.md) | Optional addon: sync values from and push plans to Scalable Capital via `sc` |

## Status

Complete: engine, CLI and TUI. `rebal-sc` is an optional addon and never a dependency of `rebal`.

## Development

```sh
make test       # go test ./... -count=1
make lint       # gofmt, go mod tidy, vet, staticcheck, govulncheck — same as CI
make snapshot   # build all release archives into ./dist (needs goreleaser)
make run ARGS="plan --months 6"
```

Changes go through pull requests with [Conventional Commit](https://www.conventionalcommits.org)
titles, squash-merged into `main`. Releases are cut by pushing a `vX.Y.Z` tag; GoReleaser builds the
archives and updates the Homebrew tap.
