---
title: Environments
description: How Orch identifies and manages environment instances.
---

An environment is one run of a manifest under a specific `env-id`.

The manifest describes the desired shape. The `env-id` identifies one concrete instance of that shape.

```sh
orch up --env-id pr-123
orch down --env-id pr-123
```

The same manifest can be used for multiple concurrent environments as long as component adapters isolate names and work directories using the environment ID.

## State Bundle

Each environment has a state bundle. The default local backend stores it at:

```text
.orch/<env-id>/
```

The bundle contains `state.json` and any artifacts that adapters need for teardown, such as Terraform state.

After a successful `down`, the environment state bundle is deleted.
