# Known issues

## `push --all` confirmation IDs may be rejected

Seen with `sc` 1.3.0: after a `push --all` with four changes, confirming the first failed with
`SAVINGS_PLAN_CONFIRMATION_NOT_FOUND`. Previewing and confirming one ISIN at a time worked for all
four. The cause is unconfirmed: `sc` may keep only the latest preview, or the ID may have expired
early. Nothing is submitted when this happens.

**Workaround:** use `push --all` as an overview, then go one position at a time:

```sh
rebal-sc push IE00B4L5Y983
rebal-sc push confirm IE00B4L5Y983 --id <CONFIRMATION_ID>
```

## `push` only compares amounts

A plan counts as in sync once its amount matches, whatever its execution day. `push` keeps the
existing day and has no option to change it.

**Workaround:** change the day in the Scalable app, or with
`sc broker savings-plans add --day-of-month`.

## The working directory decides which config is used

A `config.toml` in the current directory wins over the default one, and `state.json` is resolved
separately. Running the same command from another directory can silently use different targets
with the same values.

**Workaround:** keep a single config, or pass `--config` and `--state`.

## Ideas

- **Guided push:** walk through the changes one position at a time (preview, ask, confirm, next)
  instead of copying confirmation IDs. `sc` requires an explicit confirmation in a separate
  interaction and forbids running phase 2 automatically or in the same step as the preview, so
  check that a prompt satisfies this before building it.
- **`rebal-sc status`:** one read-only overview of the config in use, the age of the values, live
  plan total vs. `monthly_total`, execution days and next runs. Would catch a wrong config, a wrong
  monthly amount or a changed execution day before a push.
