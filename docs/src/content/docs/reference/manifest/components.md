---
title: Components
description: Manifest component fields.
---

Components are the units Orch applies, records, inspects, and destroys.

```yaml
components:
  web:
    type: docker-compose
    runner: local
    depends_on:
      - setup
    workdir: ./work/web
    source:
      files:
        compose.yml: ./docker-compose.yml
    config:
      command: docker compose
    env:
      BASE_URL: "http://localhost"
    hooks:
      post_apply:
        - command: curl -fsS "http://localhost:${web.outputs._meta.ports.services.web.80}"
```

| Field | Required | Description |
| --- | --- | --- |
| `type` | Yes | Adapter name, such as `script`, `docker-compose`, `terraform`, or `cloudformation`. |
| `runner` | Yes | Runner name from the `runners` map. |
| `depends_on` | No | Components that must apply before this component. Destroy runs in reverse state order. |
| `workdir` | No | Component work directory on the runner. Orch builds a default when omitted. |
| `source` | Adapter-specific | Component source. Each adapter decides which source modes it supports. |
| `with` | No | Supporting files copied into the component workdir but not treated as executable/source entrypoints. |
| `config` | Adapter-specific | Adapter configuration. |
| `env` | No | Environment variables passed to adapter commands and scripts. Values are interpolatable. See [Env](/reference/manifest/env/). |
| `outputs` | No | User-declared outputs expected from the adapter. |
| `hooks` | No | Lifecycle hooks run around apply and destroy. Requires a runner with `Exec`. |

Adapter-specific `config`, `source`, and output behavior is documented on each adapter page.
