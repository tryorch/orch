# Lifecycle hooks

Lifecycle hooks let a component run shell commands around apply and destroy. Hooks execute on the component's runner, so the runner must support `Exec`.

## Manifest shape

Hooks are configured on a component:

```yaml
components:
  - name: api
    type: docker-compose
    runner: local
    depends_on:
      - database
    hooks:
      pre_apply:
        - command: ./scripts/prep.sh
      post_apply:
        - command: ./scripts/check.sh "${api.outputs.url}"
          env:
            DATABASE_URL: "${database.outputs.url}"
      pre_destroy:
        - command: ./scripts/backup.sh
      post_destroy:
        - command: ./scripts/cleanup.sh
```

Each hook item has:

- `command`: required shell command
- `env`: optional environment variables for that hook command

Commands are executed through `sh -c`.

## Phases

Apply order:

```text
pre_apply
adapter apply
register outputs
capture artifacts
save state
post_apply
```

Destroy order:

```text
restore artifacts
pre_destroy
adapter destroy
mark destroyed
save state
post_destroy
```

`post_apply` can reference outputs from the current component because outputs are registered before the hook runs. `pre_apply` can only reference outputs from components that have already applied earlier in the dependency order.

Destroy hooks can reference persisted component outputs from state.

## Interpolation

Hook commands and hook environment values support the same interpolation syntax as component config:

```text
${component.outputs.name}
${INPUT_NAME}
${ENV_VAR}
```

Interpolation is strict. If a value cannot be resolved, the hook fails instead of replacing the expression with an empty string.

Outputs are only available if they were declared in the producing component's `outputs` list and returned by the adapter.

## Environment

Every component run receives these base environment variables:

- `ORCH_ENV_ID`
- `ORCH_COMPONENT_NAME`
- `ORCH_COMPONENT_TYPE`
- `ORCH_RUNNER_NAME`

Hooks additionally receive:

- `ORCH_LIFECYCLE`
- `ORCH_WORKDIR`

`ORCH_WORKDIR` is hook-specific. It is the directory used as the hook command's working directory, not a global promise about every adapter command.

Hook `env` values override component env values for that hook command. Orch-managed env values are set last.

## Failure behavior

Hooks are fail-fast. A failed interpolation, command execution error, or non-zero exit code stops the current lifecycle.

For `post_apply`, state and artifacts have already been saved before the hook runs. For `post_destroy`, the component has already been marked destroyed before the hook runs.
