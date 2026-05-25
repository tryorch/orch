---
title: Terraform
description: Apply and destroy Terraform modules.
---

The Terraform adapter copies a module to the runner, runs `terraform init`, applies it, captures outputs, and stores state needed for teardown.

Required runner capabilities:

- `Exec`
- `FileCopy`

Supported source modes:

- `embedded`
- `path`
- `files`

## Local Terraform State

If a module does not define its own Terraform backend, Orch captures local Terraform state artifacts:

- `terraform.tfstate`
- `terraform.tfstate.backup`
- `.terraform.lock.hcl`

Before destroy, Orch restores those artifacts to the runner workdir.

If the module defines a Terraform backend block, Orch treats Terraform as owning its own state and skips local tfstate artifact capture.

## Outputs

Terraform outputs are captured from `terraform output -json`.

Outputs marked sensitive by Terraform are not persisted in Orch state.

## Destroy

Destroy restores artifacts when needed, runs `terraform init`, then runs `terraform destroy -auto-approve`.
