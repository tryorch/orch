---
title: Outputs
description: Manifest output declarations and reserved metadata outputs.
---

Outputs are named objects.

```yaml
outputs:
  - name: url
  - name: token
    sensitive: true
  - name: optional_value
    required: false
```

| Field | Required | Default | Description |
| --- | --- | --- | --- |
| `name` | Yes | None | Output name. Must not be `_meta` or start with `_meta.`. |
| `required` | No | `true` | Whether apply fails if the output is missing. |
| `sensitive` | No | `false` | Sensitive outputs are available during the same `up`, but are not persisted in state. |
| `type` | No | Empty | Reserved for future output typing. |

Reference an output:

```yaml
env:
  BASE_URL: "${api.outputs.url}"
```

Adapter-generated operational outputs live under the reserved `_meta` namespace and do not need to be declared. For example, Docker Compose exposes published ports as:

```yaml
env:
  BASE_URL: "http://localhost:${web.outputs._meta.ports.services.api.80}"
```
