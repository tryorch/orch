---
title: Docker Compose
description: Run Docker Compose projects as components.
---

The Docker Compose adapter copies Compose files to the runner, runs `docker compose up -d`, captures published port metadata, and records enough state to run `docker compose down -v` later.

## Capabilities

Required runner capabilities:

| Capability | Why it is required |
| --- | --- |
| `Exec` | Runs Docker Compose commands on the runner. |
| `FileCopy` | Copies Compose files and supporting files to the runner workdir. |

The runner must have Docker and Docker Compose available.

## Sources

Supported source modes:

| Source mode | Supported | Behavior |
| --- | --- | --- |
| `files` | Yes | Each file is copied to the runner workdir and passed to Compose with `-f`. |
| `embedded` | No | Not supported. |
| `path` | No | Not supported. |

Example:

```yaml
components:
  web:
    type: docker-compose
    runner: local
    source:
      files:
        compose.yml: ./docker-compose.yml
```

Multiple files are passed to Compose in manifest map iteration order as `-f` arguments. Docker Compose merges those files into one effective project before applying.

## Config

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `command` | string | `docker compose` | Compose command to execute. Use `docker-compose` for the legacy standalone binary. |
| `flags` | list of strings | `[]` | Extra flags appended after the command and before `-f` files. |

Example:

```yaml
config:
  command: docker compose
  flags:
    - --ansi
    - never
```

## Environment

Component `env` values are passed to Compose. Orch also sets:

| Variable | Description |
| --- | --- |
| `COMPOSE_PROJECT_NAME` | Stable Orch-managed project name derived from environment ID and component name. |
| `ORCH_ENV_ID` | Current Orch environment ID. |
| `ORCH_WORKDIR` | Component workdir on the runner. |

## Outputs

Docker Compose does not have user-defined outputs. The adapter emits reserved `_meta` outputs automatically for published service ports.

For a service named `web` publishing container port `80`:

```yaml
env:
  WEB_PORT: "${web.outputs._meta.ports.services.web.80}"
  WEB_BINDING: "${web.outputs._meta.bindings.services.web.80}"
```

| Output | Example value | Description |
| --- | --- | --- |
| `_meta.ports.services.<service>.<containerPort>` | `64313` | Host port parsed from Docker Compose's published binding. |
| `_meta.bindings.services.<service>.<containerPort>` | `0.0.0.0:64313` | Raw binding returned by `docker compose port`. |

The `_meta` namespace is reserved for Orch-generated operational metadata. It does not need to be declared in `outputs`.

Port metadata is captured after `up -d` by running:

```sh
docker compose ... port <service> <containerPort>
```

These outputs describe Docker Compose's merged project, not the individual Compose files where a service or port was declared.

## Apply Behavior

Apply does the following:

1. Copies `with` files to the component workdir.
2. Copies Compose source files to the component workdir.
3. Runs `docker compose -f ... -p <project> up -d`.
4. Captures `_meta` port and binding outputs for declared Compose service ports.
5. Stores command, compose files, environment, project name, and workdir in component state.

Orch warns when fixed host ports are declared because they can conflict across concurrent environments.

## Destroy Behavior

Destroy reads component state and runs:

```sh
docker compose -f ... -p <project> down -v
```

Destroy uses the command, files, project name, environment, and workdir captured at apply time.
