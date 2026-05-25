---
title: Security Overview
description: Current security posture and boundaries.
---

Orch is an orchestration tool, not a secret manager.

The current security posture is built around a few boundaries:

- Do not persist sensitive outputs in Orch state.
- Treat state backends and artifacts as sensitive operational data.
- Prefer ambient authentication for runners and cloud providers.
- Avoid logging command invocations and environment values by default.
- Let shells expand `$ENV_VAR` on the runner instead of interpolating environment variables into command strings.

## Sensitive Outputs

Sensitive outputs are available during the same `orch up` process that produced them. They are not persisted in state.

If a later run needs a sensitive output from a skipped component, Orch should fail clearly instead of inventing or leaking a value.

## State

State can include operational handles, resource identifiers, adapter payloads, and tool artifacts such as Terraform state.

Store local state outside source control. Keep object-store state private and encrypted.

## Teardown

Today, `orch down` still needs the manifest so it can load the state backend and runner topology.

Future hosted workflows may remove that manifest dependency by loading state and runner identity from a managed control plane.
