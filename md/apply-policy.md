# Apply policy

This document describes the current Orch behavior for repeated `up` runs, partial failures, and component state. It also captures proposals we have intentionally deferred.

## Current behavior

`orch up` is state-aware. It loads the configured state backend before walking the component graph.

For each component in dependency order:

- missing from state: apply it
- `destroyed`: apply it again
- `failed` in an apply-side stage: retry it
- `failed` in a destroy-side stage: block and ask the user to run `down`
- `applying`: warn and retry it
- `destroying`: stop and ask the user to run `down` again first
- `applied`: skip it and rehydrate its persisted outputs for downstream interpolation, unless `--reapply` is set

Skipping an already applied component is the default because not every adapter is naturally idempotent. Docker Compose, Terraform, and CloudFormation can usually converge safely, but the script adapter can perform arbitrary side effects. Until Orch has fingerprinting and explicit reapply policies, the conservative behavior is to avoid rerunning live components.

`orch up --reapply` disables this skip behavior globally. It runs the normal apply flow again for already-applied components, including hooks, adapter apply, output validation, artifact capture, and final state save.

## State write points

Orch stores component state aggressively so `down` has the best available information after a failed or interrupted run.

Current `up` write points:

```text
resolve config/env
validate config
mark component applying
save state
run pre_apply hooks
run adapter apply
validate outputs
capture artifacts
mark component applied
save state
run post_apply hooks
```

If a pre-apply hook, adapter apply, output validation, artifact capture, or post-apply hook fails, Orch marks the component as `failed`, records the lifecycle `stage`, and saves state. When the adapter returned destroyable state before the failure, Orch preserves that state so `orch down` can still attempt cleanup.

Current `down` write points:

```text
restore artifacts
run pre_destroy hooks
mark component destroying
save state
run adapter destroy
mark component destroyed
save state
run post_destroy hooks
```

## Failure policy

If component `x` fails after earlier components were applied, Orch stops the apply. It does not automatically tear down earlier components.

This is intentional:

- the user may need the partial environment for debugging
- automatic cleanup can hide the original failure
- some destroys need state or artifacts that may still be in the process of being captured

Recovery is explicit:

```bash
orch up <env-id>
```

Retries failed or uncertain components.

```bash
orch down <env-id>
```

Destroys components recorded in state, in reverse order.

## Outputs on repeated up

When a component is skipped because it is already `applied`, Orch registers the outputs stored in state so later components can still interpolate values such as:

```yaml
env:
  API_URL: "${api.outputs.url}"
```

Sensitive outputs are intentionally not stored in state. This means a fresh `orch up` process cannot rehydrate sensitive outputs from an already-applied component. If a new or retried component needs a sensitive output from an already-applied component, that interpolation can fail. This is a known tradeoff until we have a secret backend or an explicit refresh/reapply policy.

Orch records this distinction in the in-memory resolver when it skips an already-applied component. Non-sensitive outputs stored in state are available for interpolation. Sensitive outputs that were declared in the manifest but dropped from state are marked as unavailable, and interpolation fails only if something actually references them.

Example:

```yaml
components:
  - name: db
    outputs:
      - name: url
      - name: password
        sensitive: true

  - name: api
    env:
      DATABASE_URL: "${db.outputs.url}"
      DATABASE_PASSWORD: "${db.outputs.password}"
```

If `db` is freshly applied in the same `orch up` process, both outputs are available in memory. If `db` was already applied in a previous run and is skipped, `url` can be rehydrated from state but `password` cannot. Orch returns an explicit error explaining that the sensitive output is unavailable because it is not persisted.

## Adapter behavior

Current repeated-up behavior is mostly orchestrator-owned:

- applied components are skipped before the adapter runs
- failed and applying components are retried through the adapter
- destroyed or missing components are applied through the adapter

Adapters still own their native apply behavior when Orch decides to call them:

- Docker Compose runs `docker compose up -d`
- Terraform runs `terraform apply`
- CloudFormation deploys the stack, allowing stack updates
- Script executes source files in the configured shell

## Future proposals

### Component fingerprints

Store a fingerprint per component:

- type
- runner
- workdir
- source hash
- non-sensitive config hash
- interpolated environment hash
- output schema
- hooks

Then `orch up` can distinguish unchanged applied components from changed ones.

### Reconcile, reapply, and replace

Possible future commands or flags:

```bash
orch up <env-id> --reconcile
orch up <env-id> --reapply component-name
orch up <env-id> --replace component-name
```

Possible behavior:

- `--reconcile`: call adapters that declare safe convergence support
- `--reapply`: run apply again without destroying first
- `--replace`: destroy from state, then apply

### Adapter convergence capabilities

Adapters may eventually declare how they handle an already-created component:

- `converge`: safe to apply again
- `run_once`: skip unless explicitly re-run
- `replace_only`: must destroy before applying changes

Initial likely classification:

- Docker Compose: `converge`
- Terraform: `converge`, if backend/artifacts are available
- CloudFormation: `converge`
- Script: `run_once` by default

### Required components

All components are currently required. A failed component fails the whole apply.

A future manifest field could allow best-effort components:

```yaml
components:
  - name: metrics
    required: false
```

Possible behavior:

- required component fails: stop apply
- optional component fails: mark failed, warn, and continue

### Rollback on failure

Default failure behavior should remain `keep`.

A future flag or manifest policy could request rollback:

```bash
orch up <env-id> --rollback-on-failure
```

or:

```yaml
policy:
  on_failure: rollback
```

Rollback should destroy only components applied during the current run, in reverse order. It should not blindly destroy pre-existing components from earlier successful runs.
