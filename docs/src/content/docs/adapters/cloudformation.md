---
title: CloudFormation
description: Create and delete CloudFormation stacks.
---

The CloudFormation adapter copies a template to the runner and uses AWS CLI commands from the runner context.

Required runner capabilities:

- `Exec`
- `FileCopy`

Supported source modes:

- `embedded`
- `files`

## Destroy

Destroy uses the stack name and workdir captured in state.

The runner must have AWS CLI available and enough ambient AWS authorization to manage the stack.

## Outputs

CloudFormation stack outputs are captured and exposed as component outputs.
