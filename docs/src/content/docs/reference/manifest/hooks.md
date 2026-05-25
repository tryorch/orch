---
title: Hooks
description: Manifest lifecycle hook fields.
---

Hooks run commands around component lifecycle phases.

```yaml
hooks:
  pre_apply:
    - command: ./prepare.sh
  post_apply:
    - command: curl -fsS "${api.outputs.url}/health"
```

Hook phases:

- `pre_apply`
- `post_apply`
- `pre_destroy`
- `post_destroy`

Each hook supports:

| Field | Required | Default | Description |
| --- | --- | --- | --- |
| `command` | Yes | None | Command string. Values are interpolatable. |
| `shell` | No | `["sh", "-c"]` | Shell command prefix used for the hook. |
| `env` | No | `{}` | Additional hook environment values. Values are interpolatable. |

Hooks require the selected runner to support `Exec`. See [Lifecycle Hooks](/concepts/lifecycle-hooks/) for behavior details.

See [Env](/reference/manifest/env/) for default runtime variables available to hooks.
