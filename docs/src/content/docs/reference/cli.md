---
title: CLI
description: Current Orch command reference.
---

## Global

```sh
orch --debug <command>
```

`--debug` enables debug logging.

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

## down

```sh
orch down --env-id <id> [--file orch.yaml] [--param key=value] [--params-file path]
```

Destroys an environment from persisted state.

`down` still needs the manifest today so Orch can load the state backend and runner topology.

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
