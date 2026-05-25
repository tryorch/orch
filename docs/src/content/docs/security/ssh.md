---
title: SSH
description: SSH runner security notes.
---

SSH host key verification is required.

Configure exactly one verification method:

```yaml
host_key:
  known_hosts: ~/.ssh/known_hosts
```

or:

```yaml
host_key:
  insecure: true
```

Use insecure mode only for local development or disposable test hosts.

## Command And Env Handling

The SSH runner sends an execution wrapper over stdin so environment values do not have to appear in the remote command string.

Orch does not intend to log raw command invocations or env values by default. User scripts can still print secrets to stdout or stderr, so treat process output as potentially sensitive.
