---
title: Manifest
description: Current alpha manifest shape.
---

The manifest format is alpha.

```yaml
version: orch.io/1.0

metadata:
  id: my-env
  description: Example environment
  owner:
    name: Platform
    email: platform@example.com

state:
  backend: local
  config:
    path: .orch

runners:
  local:
    type: local
    config: {}

components:
  setup:
    type: script
    runner: local
    source:
      embedded: |
        echo "ready=true" >> "$ORCH_OUTPUT_ENV"
    outputs:
      - name: ready
```

## Inputs

Inputs can be supplied with `--param` or `--params-file`.

```yaml
inputs:
  image:
    type: string
    default: myapp:latest
  token:
    type: string
    sensitive: true
```

Use an input:

```yaml
config:
  image: "${inputs.image}"
```

## Source

Only one source mode can be set:

- `embedded`
- `path`
- `files`

Adapters decide which source modes they support.

## With Files

`with` copies supporting files into the component runner context.

```yaml
with:
  config.json: ./config/dev.json
```

## Outputs

Outputs are named objects:

```yaml
outputs:
  - name: url
  - name: token
    sensitive: true
  - name: optional_value
    required: false
```

Outputs are required by default.
