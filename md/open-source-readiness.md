# Open source readiness plan

This is the working plan for getting Orch ready for outside contributors and early users. The goal is to keep the project moving without losing the thread on safety, recovery, and trust.

## 1. State and recovery semantics

This is the first priority.

Decisions to settle and document:

- what `orch up` does when state already exists
- how partial apply failures are represented
- how interrupted `up` and `down` runs are recovered
- which statuses block, retry, skip, or destroy
- how sensitive outputs behave across process boundaries
- how state artifacts are captured and restored
- what guarantees local and remote state backends do and do not provide

Current direction:

- `applied` components are skipped and their non-sensitive outputs are rehydrated
- `failed` and `applying` components are retried by `up`
- `destroying` blocks `up`; run `down` again to finish cleanup
- sensitive outputs are not stored in state and fail clearly when referenced from skipped components
- state is written before risky apply/destroy phases where possible

Open questions:

- whether to add a first `orch state inspect` command before more backend work
- whether `post_apply` failure should leave the component `failed` or `applied_with_failed_hook`
- whether `down` should attempt destroy for `failed` components with partial state
- how much state repair should be manual versus command-assisted

## 2. Security posture

Before broader release, do a focused pass for secret and auth handling.

Needed:

- remove command invocation leaks from runners and logs
- document that process output can still leak user-printed secrets
- avoid persisting sensitive outputs
- decide how SSH env should work without exposing secrets in process args
- replace or configure SSH host key verification instead of unconditional insecure host key behavior
- document ambient auth requirements for destroy
- document that state and artifacts can be sensitive

## 3. Adapter contract

Make the adapter lifecycle boring and explicit.

Needed:

- document apply behavior per adapter: converge, run-once, or replace-only
- define destroy as state-driven and ambient-auth-only
- standardize output validation and sensitive output handling
- standardize source and `with` file behavior
- standardize artifact declaration
- document when hooks run and what environment they receive

## 4. Manifest stability

The manifest is still alpha, but it should be predictably alpha.

Needed:

- mark manifest version as alpha in docs and examples
- keep examples aligned with the current schema
- reject ambiguous source config and unknown backend config loudly
- document outputs as named objects with `required` and `sensitive`
- document script source behavior and shell defaults

## 5. Tests and smoke coverage

Needed:

- local script smoke
- docker compose smoke
- Terraform local-state smoke
- CloudFormation opt-in smoke
- SSH runner smoke
- state resume tests
- interrupted status tests
- sensitive output tests
- backend artifact tests

## 6. CLI and release polish

Needed:

- consistent command flags
- clear recovery hints after failures
- useful `--debug`
- CI-friendly output
- `orch version`
- install/build docs
- license
- contributing guide
- GitHub CI for tests and formatting

## Near-term sequence

1. Tighten state/recovery behavior and docs.
2. Add tests around partial failure and rerun behavior.
3. Harden runner command/env handling, especially SSH.
4. Stabilize adapter contract docs.
5. Build the public README and quickstart around local-only examples.
