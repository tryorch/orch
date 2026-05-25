---
title: Source and With Files
description: Component source modes and supporting files.
---

Only one source mode can be set on a component.

| Mode | Shape | Description |
| --- | --- | --- |
| `embedded` | String | Inline source content. |
| `path` | String | Local directory path. |
| `files` | Map | Runner-side filename to local file path. |

Example:

```yaml
source:
  files:
    app.yml: ./compose.yml
```

Adapters decide which source modes are valid. See the adapter reference pages for exact support.

## With Files

`with` copies supporting files into the component runner workdir.

```yaml
with:
  config.json: ./config/dev.json
```

Unlike `source.files`, `with` files are not treated as script entrypoints or adapter primary files. They are just supporting context.
