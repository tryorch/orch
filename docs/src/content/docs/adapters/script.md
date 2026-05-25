---
title: Script
description: Run scripts as components.
---

The script adapter runs shell scripts on a runner.

Required runner capabilities:

- `Exec`
- `FileCopy`

Supported source modes:

- `embedded`
- `files`

`path` is intentionally not supported for script components.

## Example

```yaml
components:
  setup:
    type: script
    runner: local
    source:
      embedded: |
        echo "token=abc" >> "$ORCH_OUTPUT_ENV"
        echo '{"url":"http://localhost:8080"}' > "$ORCH_OUTPUT_JSON"
    outputs:
      - name: token
        sensitive: true
      - name: url
```

## Outputs

Scripts can write outputs in either format.

Environment file:

```sh
echo "name=value" >> "$ORCH_OUTPUT_ENV"
```

JSON file:

```sh
cat > "$ORCH_OUTPUT_JSON" <<'JSON'
{
  "url": "http://localhost:8080"
}
JSON
```

JSON outputs must be scalar values.

## Shell

Script components default to `sh`.

Hooks default to `["sh", "-c"]`; that is a separate hook behavior.
