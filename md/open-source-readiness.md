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
- `failed` in destroy-side stages and `destroying` block `up`; run `down` again to finish cleanup
- components persist both `status` and lifecycle `stage`
- `orch up --reapply` reruns already-applied components through the normal apply flow
- sensitive outputs are not stored in state and fail clearly when referenced from skipped components
- state is written before risky apply/destroy phases where possible

Open questions:

- whether to add a first `orch state inspect` command before more backend work
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

State and artifact security action points:

- document that Orch state is operational state, not a secret store
- document that state backends and artifacts should be treated as sensitive infrastructure data
- keep default `state inspect` table output free of outputs, payloads, and artifacts
- audit adapter payloads so secrets are not persisted accidentally
- ensure tool-state artifacts such as Terraform state are handled as sensitive even when required for teardown
- document that local `.orch` should remain ignored and object-store backends should be private and encrypted
- document that sensitive outputs are process-local until a secrets backend exists
- document that teardown should prefer ambient auth plus stable identifiers, not application secrets
- fail clearly when destroy hooks or later runs reference unavailable sensitive outputs
- defer a dedicated secrets backend design instead of smuggling secrets into state

Teardown and manifest dependency:

- current `down` still requires the manifest so Orch can configure the state backend and runner topology
- reliable teardown should not require secret material embedded in that manifest
- ambient-auth checks are runner-owned for now; adapters should not grow their own credential detector unless component/provider config starts explicitly carrying credentials
- future Orch Cloud should keep enough runtime/environment metadata to run teardown from persisted state without needing the original manifest checkout
- in that future model, teardown uses persisted state plus cloud/runtime runner identity, not apply-time manifest secrets

SSH host key verification action points:

- remove unconditional insecure SSH host key behavior
- support `host_key.known_hosts` for OpenSSH-style known hosts verification
- support `host_key.insecure: true` only as an explicit development opt-in
- require exactly one host key verification method
- emit a warning when insecure host key verification is used
- defer pinned SHA256 fingerprints and exact public key pinning until there is clear demand
- defer trust-on-first-use until there is a clearer persistence and first-connect policy

Command/env secrecy action points:

- define the policy: Orch does not log command invocations or env values by default
- audit runner, adapter, hook, script, event, debug, and error paths for command/env leakage
- keep events generic, such as `hook started`, without interpolated command text
- avoid including raw command strings or env maps in errors
- replace SSH inline env with a stdin shell wrapper so env values are not placed in remote process args
- prefer passing secrets through env over direct command interpolation in docs and examples
- do not use the environment resolver for shell command interpolation; shell commands should only interpolate explicit Orch inputs and component outputs, leaving normal `$ENV_VAR` expansion to the shell on the runner
- add tests proving SSH command strings do not contain env values and hook events do not include command contents

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
