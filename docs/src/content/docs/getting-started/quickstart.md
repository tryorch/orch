---
title: Quickstart
description: Run a local script component with Orch.
---

This quickstart runs a local script component, captures an output, inspects state, and tears the environment down.

## Build Orch

```sh
go build -o bin/orch ./cmd/orch
```

## Create A Manifest

Generate a starter manifest:

```sh
bin/orch init --id hello
```

Or create `orch.yaml` manually:


```yaml
version: orch/1.0

metadata:
  id: hello
  description: Local script example
  owner:
    name: Orch
    email: orch@example.com

runners:
  local:
    type: local
    config: {}

components:
  hello:
    type: script
    runner: local
    source:
      embedded: |
        echo "message=hello from orch" >> "$ORCH_OUTPUT_ENV"
    outputs:
      - name: message
```

## Apply

```sh
bin/orch up --env-id demo
```

Orch applies the component and writes state under `.orch/demo` by default.

## Inspect

```sh
bin/orch state inspect --env-id demo
```

The default table output shows component status, stage, type, runner, and timestamps. It intentionally does not print outputs, payloads, or artifact contents.

## Destroy

```sh
bin/orch down --env-id demo
```

After a successful destroy, Orch deletes the whole environment state bundle, including artifacts.
