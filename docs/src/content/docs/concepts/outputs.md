---
title: Outputs
description: How components expose values to later components.
---

Outputs let one component pass values to components that run later in the graph.

```yaml
components:
  producer:
    type: script
    outputs:
      - name: token
        sensitive: true

  consumer:
    type: script
    depends_on:
      - producer
    env:
      TOKEN: "${producer.outputs.token}"
```

## Declaring Outputs

Outputs are named objects.

```yaml
outputs:
  - name: url
  - name: token
    sensitive: true
  - name: optional_value
    required: false
```

Outputs are required by default.

## Sensitive Outputs

Sensitive outputs are available during the same `orch up` process that produced them, but they are not persisted in state.

If a later run skips the producer because it is already applied, sensitive outputs from that producer are unavailable. Orch should fail clearly rather than invent or leak the value.

## Script Outputs

Script components can write outputs to either file:

```sh
echo "url=http://localhost:8080" >> "$ORCH_OUTPUT_ENV"
```

or:

```sh
cat > "$ORCH_OUTPUT_JSON" <<'JSON'
{
  "url": "http://localhost:8080"
}
JSON
```
