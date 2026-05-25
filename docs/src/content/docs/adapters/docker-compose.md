---
title: Docker Compose
description: Run Docker Compose projects as components.
---

The Docker Compose adapter copies Compose files to the runner and runs `docker compose up`.

Required runner capabilities:

- `Exec`
- `FileCopy`

Supported source modes:

- `files`

## Example

```yaml
components:
  web:
    type: docker-compose
    runner: local
    workdir: ./tests/docker-compose-smoke/.workdir
    source:
      files:
        docker-compose.yml: ./docker-compose.yml
    config:
      command: docker compose
```

## Destroy

Destroy runs Compose `down -v` using the project name, workdir, command, and compose files captured in state.

Compose projects are isolated with an Orch-managed project name derived from the environment ID and component name.
