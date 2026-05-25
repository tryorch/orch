---
title: Env
description: Component environment variables and Orch runtime env defaults.
---

`env` is a string map passed to a component's adapter operations.

```yaml
components:
  smoke:
    type: script
    runner: local
    env:
      BASE_URL: "http://localhost:${web.outputs._meta.ports.services.web.80}"
```

Values are interpolated before execution. Orch warns when environment keys look like credential or access-mechanism values, but it still passes them through.

## Default Runtime Variables

Orch adds a small set of runtime variables to every component execution environment.

`ORCH_WORKDIR` is always available when an adapter command or lifecycle hook runs. It points at the component workdir on the runner for adapter operations, and at the selected hook workdir during hooks.

| Variable | Available in | Description |
| --- | --- | --- |
| `ORCH_ENV_ID` | Adapter operations, hooks | Current Orch environment ID. |
| `ORCH_COMPONENT_NAME` | Adapter operations, hooks | Current component name. |
| `ORCH_COMPONENT_TYPE` | Adapter operations, hooks | Current component adapter type. |
| `ORCH_RUNNER_NAME` | Adapter operations, hooks | Runner executing the component. |
| `ORCH_WORKDIR` | Adapter operations, hooks | Component workdir on the runner. |

Lifecycle hooks also receive:

| Variable | Available in | Description |
| --- | --- | --- |
| `ORCH_LIFECYCLE` | Hooks | Hook phase, such as `pre_apply` or `post_destroy`. |

## Adapter-Specific Variables

Adapters may add their own environment variables.

| Adapter | Variables |
| --- | --- |
| Script | `ORCH_OUTPUT_ENV`, `ORCH_OUTPUT_JSON` |
| Docker Compose | `COMPOSE_PROJECT_NAME` |

See each adapter page for adapter-specific behavior.
