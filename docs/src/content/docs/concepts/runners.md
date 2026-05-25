---
title: Runners
description: Where components execute.
---

Runners are execution targets.

Orch currently supports:

- `local`
- `ssh`

Adapters declare the capabilities they need. Runners expose capabilities such as:

- `Exec`
- `FileCopy`

Lifecycle hooks require `Exec`. Adapters that copy source files or artifacts require `FileCopy`.

## Local Runner

```yaml
runners:
  local:
    type: local
    config: {}
```

## SSH Runner

SSH runners execute through a remote shell wrapper and copy files over SSH/SFTP.

SSH host key verification is required. Configure either `known_hosts` or explicit `insecure` mode:

```yaml
runners:
  vm:
    type: ssh
    config:
      host: example.com
      user: deploy
      host_key:
        known_hosts: ~/.ssh/known_hosts
```

Use `host_key.insecure: true` only for local development or throwaway test hosts.
