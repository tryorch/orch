---
title: CLI
description: Current Orch command reference.
---

## Global

```sh
orch --debug <command>
```

`--debug` enables debug logging.

Most examples use the long flags. Short aliases are available for common options such as `-e` for `--env-id`, `-f` for `--file`, and `-o` for `--output` on `state inspect`.

## up

```sh
orch up --env-id <id> [--file orch.yaml] [--param key=value] [--params-file path] [--reapply]
```

Applies the manifest for an environment.

Flags:

- `--env-id`, `-e`: required environment ID
- `--file`, `-f`: manifest path, default `orch.yaml`
- `--param`: repeatable key-value input
- `--params-file`: YAML or env parameter file
- `--reapply`: rerun components already marked applied

When both `--params-file` and `--param` provide the same key, the CLI `--param` value wins.

## down

```sh
orch down --env-id <id> [--file orch.yaml] [--param key=value] [--params-file path]
```

Destroys an environment from persisted state.

`down` still needs the manifest today so Orch can load the state backend and runner topology.

`down` accepts the same `--param` and `--params-file` inputs as `up`, so runner and component environment pointers can be resolved during teardown.

## state inspect

```sh
orch state inspect --env-id <id> [--file orch.yaml] [--output table|json]
```

Inspects persisted state for an environment.

The table view intentionally avoids outputs, payloads, and artifact contents.

## version

```sh
orch version
```
