---
title: Terraform Environment
description: Apply Terraform modules and preserve state needed for teardown.
---

Use the Terraform adapter when a component is managed by Terraform.

```yaml
components:
  infra:
    type: terraform
    runner: local
    workdir: ./preview
    source:
      path: ./terraform
```

Orch copies the module to the runner, runs `terraform init`, then applies it.

## Local Terraform State

If the module does not define its own backend, Orch captures local Terraform state artifacts after apply and restores them before destroy.

This matters for stateless runners, where the runner workdir may not exist by the time `orch down` runs.

If the module defines a Terraform backend block, Orch treats Terraform as owning its own state and skips local tfstate artifact capture.
