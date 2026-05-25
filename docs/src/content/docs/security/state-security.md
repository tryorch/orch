---
title: State Security
description: How to think about Orch state and artifacts.
---

State is operational data. It is not a secret store, but it can still be sensitive.

State may include:

- component names and runner references
- work directories
- adapter payloads
- non-sensitive outputs
- lifecycle status and stage
- artifacts needed for teardown

Artifacts can include tool-local state such as Terraform state. Terraform state may contain sensitive values even when Orch does not treat them as component outputs.

## Recommendations

- Keep `.orch` ignored.
- Do not commit local state bundles.
- Use private buckets for object-store state.
- Enable bucket encryption for S3 state.
- Keep state backend access scoped to the environments a job needs.
- Avoid relying on application secrets in destroy hooks.
