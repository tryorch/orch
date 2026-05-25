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
```

Orch copies the Compose files into the runner workdir and runs Compose from there.

Destroy runs `docker compose down -v` using the command, workdir, files, and project name captured in state.

## When To Use It

Use Docker Compose for app services, dependencies, and local preview stacks.

Use `with` files for supporting files that should be copied into the runner context but are not the adapter's primary source.
