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

If a later run skips the producer because it is already applied, sensitive outputs from that producer are unavailable. Orch fails clearly rather than inventing or leaking the value.

## Reserved Metadata Outputs

Adapter-generated operational outputs live under the reserved `_meta` namespace.

```yaml
env:
  BASE_URL: "http://localhost:${web.outputs._meta.ports.services.web.80}"
```

Users cannot declare outputs named `_meta` or beginning with `_meta.`. Orch keeps these values available for interpolation and state because they are adapter metadata, not user-declared outputs.

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
