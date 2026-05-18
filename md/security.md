# Security

Orch is an orchestration tool that executes commands, copies files, persists operational state, and talks to remote runners. Treat its configuration and state with the same care as other infrastructure automation systems.

## Command And Environment Secrecy

Orch should not log command invocations or environment values by default.

Runner stdout and stderr are process output. If a hook, script, or provider CLI prints a secret, Orch will stream that output because it cannot reliably distinguish secret text from normal process output.

Guidelines:

- pass secrets through environment variables instead of interpolating them directly into shell commands
- prefer `$TOKEN` shell expansion over `${component.outputs.token}` inside command strings
- do not enable future command tracing in environments where commands may contain secrets
- avoid printing tool state, provider output, or environment dumps in scripts and hooks

## SSH Host Keys

SSH runners must verify host keys unless explicitly configured otherwise.

Preferred configuration:

```yaml
runners:
  ionos:
    type: ssh
    config:
      host: example.com
      port: 22
      user: deploy
      auth:
        method: key
        key_path: ~/.ssh/id_ed25519
      host_key:
        known_hosts: ~/.ssh/known_hosts
```

For local development only:

```yaml
host_key:
  insecure: true
```

Rules:

- exactly one host key verification method must be configured
- `known_hosts` uses OpenSSH-style known hosts verification
- `insecure: true` disables host key verification and should not be used for shared, CI, or production-like environments

Pinned fingerprints, direct public-key pinning, and trust-on-first-use are intentionally deferred until there is clearer demand and a stronger persistence policy.

## State And Artifacts

Orch state is operational state, not a secret store. Even when sensitive outputs are dropped, state can still contain infrastructure-sensitive information:

- component names and locations
- runner workdirs
- adapter payloads
- non-sensitive outputs
- Terraform state artifacts
- resource identifiers

State backends should be private. Object-store backends should use private buckets, least-privilege access, and encryption. Local `.orch` directories should stay out of source control.

The default `orch state inspect` table intentionally avoids outputs, payloads, and artifact contents.

## Sensitive Outputs

Sensitive outputs are process-local unless a future secrets backend exists.

They are available to downstream components during the same `orch up` process that produced them. They are not persisted in state. If a later `up` or `down` references a sensitive output from an already-applied component, Orch should fail clearly instead of inventing or leaking a value.

## Ambient Auth And Teardown

Reliable teardown assumes ambient authority on the runner.

Today, `orch down` still needs the manifest to configure the state backend and runner topology. It should not rely on secret material embedded in the manifest. Runner ambient-auth checks enforce this boundary by blocking teardown when the runner requires non-ambient credentials.

Future Orch Cloud should store enough runtime metadata to tear down from persisted state and runner identity without requiring the original manifest checkout.
