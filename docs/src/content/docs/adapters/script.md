---
title: Script
description: Run shell scripts as components.
---

The Script adapter copies script source to the runner, executes it, and captures outputs from files written by the script.

## Capabilities

Required runner capabilities:

| Capability | Why it is required |
| --- | --- |
| `Exec` | Runs scripts on the runner. |
| `FileCopy` | Copies embedded scripts, source files, with-files, and output files. |

## Sources

Supported source modes:

| Source mode | Supported | Behavior |
| --- | --- | --- |
| `embedded` | Yes | Written to `script.sh`, copied to the runner, then executed. |
| `files` | Yes | Each mapped file is copied to the runner and executed sequentially. |
| `path` | No | Intentionally not supported for scripts. Use `files` for executable scripts and `with` for supporting files. |

Example:

```yaml
components:
  setup:
    type: script
    runner: local
    source:
      embedded: |
        echo "token=abc" >> "$ORCH_OUTPUT_ENV"
```

For `source.files`, map runner-side script names to local files:

```yaml
source:
  files:
    01-setup.sh: ./scripts/setup.sh
    02-check.sh: ./scripts/check.sh
```

Scripts from `source.files` are executed sequentially after copy.

## Config

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `shell` | list of strings | `["sh"]` | Shell command prefix used to run each script. |

Example:

```yaml
config:
  shell: ["bash"]
```

The adapter runs each script as:

```text
<shell...> ./<script>
```

Hooks use a different default: `["sh", "-c"]`.

## Environment

Component `env` values are passed to the script. The adapter also sets:

| Variable | Description |
| --- | --- |
| `ORCH_OUTPUT_ENV` | Relative path to the env-style output file. Defaults to `.orch-outputs.env`. |
| `ORCH_OUTPUT_JSON` | Relative path to the JSON output file. Defaults to `.orch-outputs.json`. |

## Outputs

Script outputs must be declared in the component's `outputs` list unless they are reserved `_meta` outputs.

Environment file format:

```sh
echo "name=value" >> "$ORCH_OUTPUT_ENV"
```

Multiple `echo` calls are supported. If the same key is written more than once, the last value wins.

JSON file format:

```sh
cat > "$ORCH_OUTPUT_JSON" <<'JSON'
{
  "url": "http://localhost:8080",
  "enabled": true,
  "port": 8080
}
JSON
```

JSON outputs must be scalar values: string, number, boolean, or null.

Example declaration:

```yaml
outputs:
  - name: token
    sensitive: true
  - name: url
```

Sensitive outputs are available during the same `orch up`, but are not persisted in state.

## Apply Behavior

Apply does the following:

1. Creates the component workdir on the runner.
2. Copies script source to the runner.
3. Copies `with` files to the runner workdir.
4. Clears previous `.orch-outputs.env` and `.orch-outputs.json`.
5. Executes scripts sequentially.
6. Copies output files back to Orch and parses outputs.
7. Stores script list, shell, and workdir in component state.

## Destroy Behavior

Script components do not have adapter-specific destroy behavior. Use `pre_destroy` and `post_destroy` lifecycle hooks for teardown commands.
