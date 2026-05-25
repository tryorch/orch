---
title: CI Environments
description: Run Orch from stateless job runners.
---

CI runners are often stateless. The job that applies an environment may not be the job that destroys it.

For CI, prefer:

- a stable `env-id`, such as a pull request number
- a shared state backend, such as S3
- ambient cloud authentication, such as OIDC
- manifests that avoid embedding credentials

## State Backend

Use an object-store backend when local state will not survive between jobs.

```yaml
state:
  backend: s3
  config:
    bucket: my-orch-state
    prefix: previews
    region: eu-central-1
```
