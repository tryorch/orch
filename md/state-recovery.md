# State recovery

Orch state records both a component `status` and the lifecycle `stage` where that status was last captured.

`status` answers what Orch believes about the component outcome. `stage` answers where Orch was in the lifecycle when that outcome was written.

## Statuses

| Status | Meaning | `orch up` behavior | `orch down` behavior |
| --- | --- | --- | --- |
| `applying` | Orch started an apply flow and did not complete it cleanly. | Retry with a warning. | Attempt destroy if state is available. |
| `applied` | Orch believes the component is live and destroyable. | Skip by default; reapply with `--reapply`. | Attempt destroy. |
| `failed` | Orch recorded a failure. Resources may or may not exist. | Retry if the stage is apply-side; block if the stage is destroy-side. | Attempt destroy if state is available. |
| `destroying` | Orch started a destroy flow and did not complete it cleanly. | Block; run `down` again first. | Retry destroy. |
| `destroyed` | Orch completed destroy. | Apply again if needed. | Skip. |

`skipped` is not a persisted status. Skipping is an event from a single `up` run, not durable component state.

## Stages

Current stages:

```text
config
pre_apply
apply
outputs
artifacts
post_apply
pre_destroy
destroy
post_destroy
```

Apply-side stages:

```text
config
pre_apply
apply
outputs
artifacts
post_apply
```

Destroy-side stages:

```text
pre_destroy
destroy
post_destroy
```

## Recovery Rules

`orch up`:

- missing component state: apply
- `destroyed`: apply
- `applied`: skip unless `--reapply` is set
- `applying`: retry
- `failed` in an apply-side stage: retry
- `failed` in a destroy-side stage: block and ask the user to run `down`
- `destroying`: block and ask the user to run `down`

`orch down`:

- `destroyed`: skip
- all other statuses: attempt destroy in reverse state order

This means `failed` does not mean "nothing exists." It means "Orch failed while doing work." If the component has enough state for destroy, `down` should try to clean it up.

## Reapply

`orch up --reapply` disables the default skip for `applied` components.

Reapply:

- walks the graph in normal order
- runs `pre_apply` hooks again
- calls adapter apply again
- refreshes outputs
- captures artifacts again
- runs `post_apply` hooks again

It does not destroy first. For adapters like Docker Compose, Terraform, and CloudFormation this usually means converge/update. For `script`, this reruns the script and can repeat side effects, so users should only use `--reapply` when their script components are safe to rerun.

## Hook Events

Lifecycle hooks emit events with the hook stage. In TTY output, a hook event is rendered as:

```text
db.pre_apply
db.post_apply
db.pre_destroy
db.post_destroy
```

Hook failure stops the graph immediately and records:

```text
status: failed
stage: <hook stage>
```

## Failure Examples

If output validation fails after adapter apply:

```json
{
  "status": "failed",
  "stage": "outputs"
}
```

The component may already exist, so `down` should attempt cleanup.

If artifact capture fails:

```json
{
  "status": "failed",
  "stage": "artifacts"
}
```

This is treated as failed because destroy may be unsafe without tool state artifacts.

If destroy fails:

```json
{
  "status": "failed",
  "stage": "destroy"
}
```

The next `up` blocks. The next `down` retries cleanup.

## Sensitive Outputs

Sensitive outputs are available in memory during the apply process that produced them. They are not stored in state.

When `up` skips an already-applied component, non-sensitive outputs are rehydrated from state. Sensitive outputs declared by that component are recorded as unavailable. If a downstream component references one, interpolation fails with an explicit error.

The future answer is either a secrets backend or an explicit reapply/replace workflow. For now, `--reapply` can be used when the producer is safe to run again.

State is not a secret store. It should remember the operational handles needed for recovery and teardown, such as names, workdirs, resource IDs, stack names, project names, and tool-state artifacts. It should not remember application secrets unless a future secrets backend explicitly provides that behavior.

Destroy flows should prefer ambient authentication and stable non-secret identifiers. A destroy hook that depends on a sensitive output from another component may work during the same process that produced that output, but it cannot be recovered from persisted state today. In a later process, Orch should fail clearly rather than silently inventing or leaking a secret.

This is a deliberate boundary:

- state backend: operational state needed to find and destroy resources
- future secrets backend: sensitive output values that must survive across processes
- destroy contract: ambient auth plus operational identifiers wherever possible

## State Save Failures

If an adapter apply succeeds but Orch cannot save state, Orch returns a loud error. Resources may exist without recoverable state.

Future work may add an emergency local recovery file, but the current behavior is intentionally simple and explicit.

## Future State Repair

Manual repair commands are not implemented yet.

The current state command is read-only:

```bash
orch state inspect -e <env-id>
orch state inspect -e <env-id> --output json
```

The default table view shows component status, stage, type, runner, and timestamps. It intentionally does not print outputs, payload, or artifact contents.

Possible future repair commands:

```bash
orch state mark-destroyed <component>
orch state rm-component <component>
```

For now, normal recovery paths are:

- killed during `up`: run `orch up` again
- killed during `down`: run `orch down` again
- failed during destroy: run `orch down` again

## Manifest Dependency

Today, `orch down` still requires the manifest. The manifest tells Orch how to load the state backend and how to reach the runners. State tells Orch what components were applied and what payload/artifacts are needed for adapter teardown.

This does not mean teardown should rely on credentials embedded in the manifest. The intended recovery model is:

- manifest: backend and runner topology
- state: destroyable component facts
- runner environment: ambient authority to reach the runner and providers

Runner ambient-auth checks enforce this boundary today. If a runner uses non-ambient credentials from the manifest, `down` blocks by default because teardown would depend on apply-time secret material.

Future Orch Cloud should remove this manifest dependency for teardown by storing enough runtime metadata with the environment. In that model, teardown can run from cloud-managed state and runner identity without checking out the original manifest.
