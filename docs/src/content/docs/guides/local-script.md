---
title: Local Script Environment
description: Run a local script component and capture outputs.
---

Use the script adapter when you need custom setup, checks, glue code, or simple automation.

```yaml
version: orch.io/1.0

metadata:
  id: script-example
  description: Local script environment
  owner:
    name: Orch
    email: orch@example.com

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
        echo "message=hello from orch" >> "$ORCH_OUTPUT_ENV"
    outputs:
      - name: message
```

Apply it:

```sh
orch up --env-id script-demo
```

Inspect state:

```sh
orch state inspect --env-id script-demo
```

Destroy it:

```sh
orch down --env-id script-demo
```
