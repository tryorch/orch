---
title: Terraform
description: Apply and destroy Terraform modules.
---

The Terraform adapter stages a module, copies it to the runner, runs Terraform, captures outputs, and preserves local Terraform state artifacts when the module does not define its own backend.

## Capabilities

Required runner capabilities:

| Capability | Why it is required |
| --- | --- |
| `Exec` | Runs Terraform commands on the runner. |
| `FileCopy` | Copies modules, with-files, and Terraform state artifacts. |

The runner must have Terraform available.

## Sources

Supported source modes:

| Source mode | Supported | Behavior |
| --- | --- | --- |
| `embedded` | Yes | Written into a local staged module and copied to the runner. |
| `path` | Yes | Directory path to the Terraform module. |
| `files` | No | Not currently supported by the Terraform adapter. |

Example:

```yaml
components:
  infra:
    type: terraform
    runner: local
    source:
      path: ./infra
```

The adapter excludes `.terraform`, `terraform.tfstate`, and `terraform.tfstate.backup` when staging a module for copy.

## Config

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `vars` | map of strings | `{}` | Terraform variable values passed with repeated `-var key=value` arguments. |

Example:

```yaml
config:
  vars:
    region: eu-central-1
    instance_type: t3.micro
```

If a Terraform variable has no default and is not present in `vars`, Orch checks for an environment variable with the same name and uses that value. If no value is found, Orch emits a warning.

## Environment

Component `env` values are passed to:

- `terraform init`
- `terraform apply`
- `terraform output -json`
- `terraform destroy`

Use `env` for provider environment variables when needed, but prefer ambient runner authentication where possible.

## Outputs

Terraform outputs are captured from:

```sh
terraform output -json
```

Terraform outputs marked sensitive by Terraform are ignored by Orch and are not persisted in Orch state.

Non-sensitive Terraform outputs must still be declared in the component `outputs` list to be available for interpolation:

```yaml
outputs:
  - name: url
```

Output values are converted to strings. Scalar values, lists, and maps are stringified by Orch's output conversion helpers.

## State Artifacts

If the module does not define a Terraform backend block, Orch captures local Terraform state artifacts after apply:

| Artifact | Required | Sensitive | Description |
| --- | --- | --- | --- |
| `terraform.tfstate` | Yes | Yes | Required local Terraform state used for destroy. |
| `terraform.tfstate.backup` | No | Yes | Backup state file when present. |
| `.terraform.lock.hcl` | No | No | Provider lock file. |

Before destroy, Orch restores captured artifacts to the component workdir.

If the module defines a backend block, Orch treats Terraform as owning its own remote state and skips local state artifact capture.

## Apply Behavior

Apply does the following:

1. Copies `with` files to the runner workdir.
2. Stages and copies the Terraform module to the runner.
3. Runs `terraform init -upgrade`.
4. Runs `terraform apply -auto-approve`, appending configured `vars`.
5. Runs `terraform output -json` and captures non-sensitive outputs.
6. Stores vars and workdir in component state.
7. Captures local Terraform artifacts when no Terraform backend is configured.

## Destroy Behavior

Destroy does the following:

1. Restores captured artifacts when they exist.
2. Re-stages and copies the Terraform module to the runner.
3. Runs `terraform init -upgrade`.
4. Runs `terraform destroy -auto-approve`, appending vars captured in state.

Destroy uses the component environment from the current manifest and the Terraform vars captured at apply time.
