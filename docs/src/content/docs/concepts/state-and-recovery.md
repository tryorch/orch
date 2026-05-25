---
title: State and Recovery
description: How Orch persists progress and recovers from failures.
---

Orch stores operational state so it can recover from interrupted or failed runs.

State is not a secret store. It records handles needed for recovery and teardown: component names, runners, work directories, adapter payloads, outputs that are safe to persist, lifecycle status, and artifacts.

## Status And Stage

Each component records both a `status` and a `stage`.

Common statuses:

- `applying`
- `applied`
- `destroying`
- `destroyed`
- `failed`

Common stages:

- `config`
- `pre_apply`
- `apply`
- `outputs`
- `artifacts`
- `post_apply`
- `pre_destroy`
- `destroy`
- `post_destroy`

The status says what happened. The stage says where it happened.

## Re-running Up

By default, `orch up` skips components already marked `applied` and rehydrates their non-sensitive outputs for downstream interpolation.

Use `--reapply` to run already-applied components again:

```sh
orch up --env-id demo --reapply
```

## Failed Runs

Apply-side failures can usually be retried with `orch up`.

Destroy-side failures block `up`; run `orch down` again so Orch can finish cleanup.

## Successful Down

After every component and post-destroy hook succeeds, `orch down` deletes the environment state bundle.

If teardown fails before completion, state is kept so the next `down` can retry.
