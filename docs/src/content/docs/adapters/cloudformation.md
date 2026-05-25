---
title: CloudFormation
description: Create and delete CloudFormation stacks.
---

The CloudFormation adapter copies a template to the runner and uses the AWS CLI from the runner context to deploy, inspect, and delete stacks.

## Capabilities

Required runner capabilities:

| Capability | Why it is required |
| --- | --- |
| `Exec` | Runs AWS CLI commands on the runner. |
| `FileCopy` | Copies templates and with-files to the runner workdir. |

The runner must have AWS CLI available and enough AWS authorization to manage the stack.

## Sources

Supported source modes:

| Source mode | Supported | Behavior |
| --- | --- | --- |
| `embedded` | Yes | Written to `template.yml`, copied to the runner, and deployed. |
| `files` | Yes | Requires exactly one template file. The file is copied to the runner and deployed. |
| `path` | No | Not supported. |

Example:

```yaml
components:
  stack:
    type: cloudformation
    runner: local
    source:
      files:
        template.yml: ./template.yml
```

## Config

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| `stack_name` | string | `orch-<env-id>-<component>` | CloudFormation stack name. |
| `region` | string | AWS CLI default | AWS region passed with `--region` when set. |
| `parameters` | map of strings | `{}` | Values passed to `--parameter-overrides` as `Key=Value`. |
| `capabilities` | list of strings | `[]` | Values passed to `--capabilities`, such as `CAPABILITY_IAM`. |
| `tags` | map of strings | `{}` | Values passed to `--tags` as `Key=Value`. |
| `role_arn` | string | Empty | IAM role ARN passed with `--role-arn`. |

Example:

```yaml
config:
  stack_name: preview-api
  region: eu-central-1
  capabilities:
    - CAPABILITY_IAM
  parameters:
    ImageTag: latest
  tags:
    Environment: preview
```

## Environment

Component `env` values are passed to AWS CLI commands. Use this for AWS environment variables only when needed. Prefer runner-local or ambient AWS authentication for destroy reliability.

## Outputs

CloudFormation outputs are captured after deploy with:

```sh
aws cloudformation describe-stacks \
  --stack-name <stack> \
  --query Stacks[0].Outputs \
  --output json
```

The adapter maps each `OutputKey` to its `OutputValue`.

Outputs must be declared in the component `outputs` list to be available for interpolation:

```yaml
outputs:
  - name: PublicURL
```

If the stack has no outputs, Orch treats that as an empty output set.

## Apply Behavior

Apply does the following:

1. Copies `with` files to the runner workdir.
2. Resolves the CloudFormation template from `embedded` or `files`.
3. Copies the template to the runner workdir.
4. Runs `aws cloudformation deploy --no-fail-on-empty-changeset`.
5. Runs `aws cloudformation describe-stacks` and captures stack outputs.
6. Stores region, stack name, template file, and workdir in component state.

## Destroy Behavior

Destroy reads component state and runs:

```sh
aws cloudformation delete-stack --stack-name <stack>
aws cloudformation wait stack-delete-complete --stack-name <stack>
```

If `region` was configured during apply, destroy passes the same region.
