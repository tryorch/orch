---
title: Docker Compose Environment
description: Run a Compose project as an Orch component.
---

Use the Docker Compose adapter when a component is described by one or more Compose files.

```yaml
components:
  web:
    type: docker-compose
    runner: local
    workdir: ./preview
    source:
      files:
        docker-compose.yml: ./docker-compose.yml
    config:
      command: docker compose

  smoke:
    type: script
    runner: local
    depends_on:
      - web
    env:
      BASE_URL: "http://localhost:${web.outputs._meta.ports.services.web.80}"
    source:
      embedded: |
        curl -fsS "$BASE_URL/health"
```

Orch copies the Compose files into the runner workdir and runs Compose from there.

## Published Ports

Docker Compose does not define user outputs, but Orch automatically captures published port metadata under `_meta`.

```yaml
env:
  WEB_PORT: "${web.outputs._meta.ports.services.web.80}"
  WEB_BINDING: "${web.outputs._meta.bindings.services.web.80}"
```

`_meta.ports...` gives just the host port. `_meta.bindings...` gives the raw binding returned by Docker Compose.

## When To Use It

Use Docker Compose for app services, dependencies, and local preview stacks.

Use `with` files for supporting files that should be copied into the runner context but are not the adapter's primary source.

## Teardown

Destroy runs `docker compose down -v` using the command, workdir, files, and project name captured in state.
