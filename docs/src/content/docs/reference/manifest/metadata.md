---
title: Metadata
description: Descriptive manifest metadata fields.
---

`metadata` is descriptive. It is not currently required for local execution.

```yaml
metadata:
  id: my-env
  description: Example environment
  owner:
    name: Platform
    email: platform@example.com
  labels:
    team: platform
```

| Field | Required | Description |
| --- | --- | --- |
| `id` | No | Human-readable environment identifier. The CLI `--env-id` controls the runtime environment ID. |
| `description` | No | Short description of the environment. |
| `owner.name` | No | Owner name. |
| `owner.email` | No | Owner email. |
| `labels` | No | Free-form string labels. |
