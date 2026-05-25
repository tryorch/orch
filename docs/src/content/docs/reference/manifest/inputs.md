---
title: Inputs
description: Manifest inputs and interpolation.
---

Inputs are values that can be interpolated elsewhere in the manifest.

```yaml
inputs:
  image:
    type: string
    default: myapp:latest
  token:
    type: string
    sensitive: true
    required: true
```

Use an input with interpolation:

```yaml
config:
  image: "${inputs.image}"
```

| Field | Required | Default | Description |
| --- | --- | --- | --- |
| `type` | Yes | None | Input type. Current examples use `string`. |
| `description` | No | Empty | Human-readable input description. |
| `default` | No | Empty | Value used when no parameter is supplied. |
| `sensitive` | No | `false` | Marks the input as sensitive for display and storage decisions. |
| `required` | No | `false` | Requires a value from params or default. |

Inputs can be provided with `--param` or `--params-file`.
