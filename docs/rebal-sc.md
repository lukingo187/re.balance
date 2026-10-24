# rebal-sc

`rebal` itself stays offline, no broker dependency. `rebal-sc` is an
optional, opt-in addon binary that bridges `rebal` and the
[official Scalable Capital broker CLI](https://github.com/ScalableCapital/scalable-cli) (`sc`):
pulling live valuations into `state.json`, and previewing/submitting `rebal plan`'s computed
amounts as Scalable savings-plan changes, with the broker's own two-step disclosure and
confirmation on every change. It reads and writes the same `config.toml` / `state.json` as
`rebal` — no separate config of its own.

## Install

Included in the Homebrew cask and release archives. From source:

```sh
go install github.com/lukingo187/re.balance/cmd/rebal-sc@latest
# or, from a checkout
make install-sc
```

Requires the [`sc` CLI](https://github.com/ScalableCapital/scalable-cli) on `PATH` and logged in
(`sc login`, a manual, human-only step — never scripted). Not part of `make build`/`make install`;
`rebal` never depends on this binary or on `sc`.

Tested against `sc` 1.2.0. CI's `sc-contract` job checks the commands, flags and two-phase
workflow `rebal-sc` relies on against the pinned version, and Renovate opens a PR on every `sc`
release. A red check on that PR means the release affects `rebal-sc`. To run it locally:
`SC_CONTRACT=1 go test ./cmd/rebal-sc -run TestSCContract`.

## Quickstart

```sh
# 1. See your Scalable holdings, savings plans and watchlist in one view.
rebal-sc list

# 2. Pull current valuations into rebal's state.json.
rebal-sc sync --all
# First time pointing at an account rebal doesn't know about yet? Also
# create positions for holdings it hasn't seen before:
rebal-sc sync --all --create-missing

# 3. Preview rebal's plan as Scalable savings-plan changes (never submits).
rebal-sc push --all

# 4. Review the disclosure, then submit one ISIN as a separate, deliberate step.
rebal-sc push confirm IE00B4L5Y983 --id <CONFIRMATION_ID>
```

## Commands

| Command | What it does |
|---|---|
| `list` | Read-only view of Scalable holdings, savings plans and watchlist. Never writes. |
| `sync [ISIN...] \| --all [--create-missing] [--dry-run]` | Pull live valuations into `state.json`. `--create-missing` also adds untracked holdings. |
| `push [ISIN...] \| --all [--amount N]` | Preview rebal's plan as Scalable savings-plan changes. Never submits or removes anything. |
| `push confirm <ISIN> --id ID [--amount N]` | Submit a previewed amount change — the only command that changes anything on Scalable. |
| `push confirm <ISIN> --remove` | Remove a frozen position's still-active Scalable plan. |

Global flags: `--config PATH`, `--state PATH`, `--no-color` — same meaning as `rebal`'s.

## Safety rules

Real money is involved, so `rebal-sc` follows `sc`'s own rules strictly:

- **Preview and submit are separate commands.** `sc` requires the cost disclosure (phase 1) and the
  confirmation (phase 2) to be separate, explicit steps. `push` only ever previews; `push confirm`
  is the only command that changes anything, and you run it yourself.
- **`push confirm` recomputes the plan** instead of trusting the preview. If anything changed in
  between, the amount no longer matches and `sc` rejects the stale confirmation.
- **Removal is its own flag.** `sc` has no preview step for removing a plan, so `--remove` is a
  separate, explicitly named path and only works for positions frozen in `rebal`.
- **Non-monthly plans are skipped** with a note, never silently reinterpreted as monthly.
- **Nothing runs unattended.** `sc login` stays manual, and there is no scheduled sync or push.

## Good to know

- **`sc` needs the Mac unlocked.** It signs every request with a Secure Enclave key, so every call
  fails while the machine is locked.
- **Confirm right after previewing.** Confirmation IDs can expire before their displayed expiry.
  A stale ID is rejected cleanly, but running `push` and `push confirm` back to back is the reliable
  path.
- **If a confirm's outcome is unknown** (timeout, crash), check `sc broker savings-plans` before
  retrying.
