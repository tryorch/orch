---
title: Components
description: Components are the units Orch applies and destroys.
---

Components are the units of work in an Orch environment.

Each component has:

- a `name`
- a `type`, which selects an adapter
- a `runner`
- optional dependencies
- optional source files
- optional config
- optional outputs
- optional lifecycle hooks

```yaml
components:
  setup:
    type: script
    runner: local
    source:
      embedded: |
        echo "token=abc" >> "$ORCH_OUTPUT_ENV"
    outputs:
      - name: token
        sensitive: true
```

## Dependencies

Components run in dependency order using `depends_on`.

```yaml
components:
  api:
    depends_on:
      - database
```

Destroy runs in reverse state order.

## Outputs

Components can expose outputs to later components:

```yaml
env:
  DATABASE_URL: "${database.outputs.url}"
```

Sensitive outputs are available during the same `up` process, but they are not persisted in state.
