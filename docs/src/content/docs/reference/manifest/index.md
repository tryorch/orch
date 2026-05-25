---
title: Manifest
description: The core Orch manifest structure.
---

The manifest describes an environment: inputs, state backend, runners, and the components Orch applies and destroys.

```yaml
version: orch.io/1.0

metadata:
  id: my-env
  description: Example environment
  owner:
    name: Platform
    email: platform@example.com

state:
  backend: local
  config:
    path: .orch

inputs:
  image:
    type: string
    default: myapp:latest

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
        echo "ready=true" >> "$ORCH_OUTPUT_ENV"
    outputs:
      - name: ready
```

The manifest format is still alpha. Field names and adapter behavior may change before a stable release.

## Top-Level Fields

| Field | Required | Description |
| --- | --- | --- |
| `version` | Yes | Manifest version. Current examples use `orch.io/1.0`. |
| `metadata` | No | Environment metadata. Useful for humans and future remote state backends. |
| `state` | No | State backend selection. Defaults to the local backend when omitted. |
| `inputs` | No | Named values that can be supplied by flags, params files, or defaults. |
| `runners` | Yes | Execution targets available to components. |
| `components` | Yes | Ordered component declarations. Orch builds a dependency graph from this list. |

## Field Reference

- [Metadata](/reference/manifest/metadata/)
- [Inputs](/reference/manifest/inputs/)
- [State](/reference/manifest/state/)
- [Runners](/reference/manifest/runners/)
- [Components](/reference/manifest/components/)
- [Source and With Files](/reference/manifest/source-and-files/)
- [Env](/reference/manifest/env/)
- [Outputs](/reference/manifest/outputs/)
- [Hooks](/reference/manifest/hooks/)

Adapter-specific `config` fields are documented on each adapter page:

- [Script](/adapters/script/)
- [Docker Compose](/adapters/docker-compose/)
- [Terraform](/adapters/terraform/)
- [CloudFormation](/adapters/cloudformation/)
