---
title: Lifecycle Hooks
description: Run commands before and after component apply or destroy.
---

Components can define lifecycle hooks:

- `pre_apply`
- `post_apply`
- `pre_destroy`
- `post_destroy`

```yaml
components:
  api:
    type: script
    runner: local
    hooks:
      post_apply:
        - command: test -n "$API_URL"
          env:
            API_URL: "${api.outputs.url}"
```

Hooks require a runner with `Exec`.

## Shell

Hooks default to:

```yaml
shell: ["sh", "-c"]
```

The command is passed as the script argument to the shell. This is different from script components, where the shell is the executable used to run a source file.

## Environment

Hook env values can use Orch interpolation. Hook commands can interpolate explicit inputs and component outputs, but normal `$ENV_VAR` expansion is left to the shell on the runner.

Orch also sets component runtime variables during hook execution:

- `ORCH_ENV_ID`
- `ORCH_COMPONENT_NAME`
- `ORCH_COMPONENT_TYPE`
- `ORCH_RUNNER_NAME`
- `ORCH_WORKDIR`
- `ORCH_LIFECYCLE`
