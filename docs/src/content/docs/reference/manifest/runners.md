---
title: Runners
description: Manifest runner declarations.
---

Runners are execution targets. Components reference them by name.

```yaml
runners:
  local:
    type: local
    config: {}

  box:
    type: ssh
    config:
      host: example.com
      port: 22
      user: root
      auth:
        method: password
        password: "${inputs.ssh_password}"
```

| Field | Required | Description |
| --- | --- | --- |
| `type` | Yes | Runner type, such as `local` or `ssh`. |
| `config` | No | Runner-specific configuration. |
| `providers` | No | Provider-specific environment/bootstrap settings translated into runner environment. |

Adapters declare required runner capabilities. For example, all current adapters require `Exec`, and adapters that copy source files also require `FileCopy`.

## Local

The local runner executes commands on the same machine as the Orch process.

```yaml
runners:
  local:
    type: local
    config: {}
```

The local runner has no config fields today.

## SSH

The SSH runner executes commands over SSH and copies files over SFTP.

```yaml
runners:
  box:
    type: ssh
    config:
      host: example.com
      port: 22
      user: deploy
      auth:
        method: key
        key_path: ~/.ssh/id_ed25519
      host_key:
        known_hosts: ~/.ssh/known_hosts
```

Config fields:

| Field | Required | Default | Description |
| --- | --- | --- | --- |
| `host` | Yes | None | SSH host name or address. |
| `port` | No | `0` | SSH port. Set `22` explicitly in manifests today. |
| `user` | Yes | None | SSH username. |
| `auth.method` | Yes | None | `password` or `key`. |
| `auth.password` | For password auth | Empty | Password value. Prefer injecting it from inputs or the environment, not hard-coding it. |
| `auth.key_path` | For key auth | Empty | Local private key path read by Orch. |
| `host_key.known_hosts` | One host key mode required | Empty | Known hosts file used for host key verification. |
| `host_key.insecure` | One host key mode required | `false` | Disables host key verification. Use only for disposable development hosts. |

Exactly one host key mode must be configured: `host_key.known_hosts` or `host_key.insecure`.

## Providers

`providers` is runner-level provider bootstrap configuration. Orch translates provider settings into runner environment values before adapter commands run.

Provider values can grant access to cloud APIs. Orch warns when provider settings use non-ambient credentials because those values may be unavailable during later teardown unless the same manifest inputs are supplied again.
