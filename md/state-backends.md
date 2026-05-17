# State backends

Orch has one state backend per manifest. The backend stores the Orch environment state document and any adapter-declared tool-state artifacts needed for teardown.

## Manifest shape

```yaml
state:
  backend: local
  config:
    path: .orch
```

If `state` is omitted, Orch defaults to the local backend at `.orch`.

```yaml
state:
  backend: s3
  config:
    bucket: my-orch-state
    prefix: previews
    region: us-east-1
    server_side_encryption: aws:kms
    kms_key_id: alias/orch-state
```

The factory only dispatches to a backend implementation. Each backend owns its own config decoding and validation so config growth stays local to the backend.

## Storage layout

Backends should preserve this logical layout:

```text
<root-or-prefix>/<env-id>/state.json
<root-or-prefix>/<env-id>/artifacts/<component-name>/<artifact-path>
```

For local:

```text
.orch/pr-123/state.json
.orch/pr-123/artifacts/tf/terraform.tfstate
```

For S3:

```text
s3://my-orch-state/previews/pr-123/state.json
s3://my-orch-state/previews/pr-123/artifacts/tf/terraform.tfstate
```

## Write ordering

Orch writes component state before and after risky lifecycle steps. Before adapter apply, the component is marked `applying` and the state document is saved. After a successful apply, Orch builds the final component state in memory, captures declared artifacts first, and then writes `state.json` with status `applied`.

This ordering is intentional. A newly written state document should not reference artifacts that failed to persist.

Successful apply order:

```text
mark component applying
save state.json
apply component
build component state
capture artifacts
mark component applied
save state.json
```

This is not fully transactional. A failure after apply but before final `state.json` can still leave resources alive with a component marked `applying` or `failed`. A future remote backend should add revisions or commit markers if we need stronger guarantees.

## Artifacts

Adapters declare artifacts when tool-local state is required for destroy.

Rules:

- artifact paths are relative to the component workdir
- absolute paths are rejected
- `../` escapes are rejected
- artifacts are captured from and restored to the same path
- artifacts are files, not directories
- local artifact files are written with `0600`

Terraform is currently the adapter that needs this. With Terraform's default local backend, `terraform.tfstate` must be preserved or `terraform destroy` cannot know what resources exist on a stateless runner.

If a Terraform module defines its own backend block, Orch treats Terraform as owning its state and skips local tfstate artifact capture.

```hcl
terraform {
  backend "s3" {}
}
```

## Temporary files

Artifact capture and restore use temporary local files as a bridge:

```text
capture: runner path -> temp local file -> state backend
restore: state backend -> temp local file -> runner path
```

This exists because the current runner API and backend API are both path-based. We cannot assume direct runner-to-backend streaming, especially once remote runners and object-store backends are involved.

The temp files are created with `os.CreateTemp("", "orch-artifact-*")`, which means the OS temp directory is used and `*` is replaced by Go with a random suffix. Files are removed after each artifact operation.

## Local backend

Config:

```yaml
state:
  backend: local
  config:
    path: .orch
```

`path` is optional and defaults to `.orch`.

Unknown local config keys are rejected.

## S3 backend

Config:

```yaml
state:
  backend: s3
  config:
    bucket: my-orch-state
    prefix: previews
    region: us-east-1
    server_side_encryption: AES256
```

Fields:

- `bucket`: required
- `prefix`: optional
- `region`: optional; ambient AWS config may provide it
- `server_side_encryption`: optional, either `AES256` or `aws:kms`
- `kms_key_id`: optional, requires `server_side_encryption: aws:kms`

Unknown S3 config keys are rejected.

The S3 backend uses ambient AWS authentication through the AWS SDK default config chain. In CI, prefer OIDC/federated identity over static credentials.

S3 state buckets should be private. Because artifacts may contain Terraform state, bucket policy and encryption are part of the safety model, not nice-to-have decoration.

## Delete semantics

Destroy marks components as destroyed and saves the state document. It does not delete the state bundle. Keeping state after destroy is useful for debugging and audit.

Future work can add an explicit state cleanup command:

```bash
orch state rm --env-id pr-123
```

## Locking and revisions

The current backend interface does not implement locking or optimistic revisions.

This is acceptable for the first local and S3 implementation, but CI can run concurrent jobs for the same `env-id`. Before we rely on object-store state heavily, we should add one of:

- optimistic revisions, where save checks an expected generation
- lock objects with expiry
- a stronger backend-specific lock mechanism

The important future rule: two concurrent `orch up` jobs should not silently overwrite each other's state for the same environment.
