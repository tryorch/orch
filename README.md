# Orch

Orch is an alpha-stage orchestration tool for ephemeral preview, test, and development environments.

It reads a manifest, runs components on local or remote runners, captures operational state, and tears environments down from that state.

## Status

Orch is early and moving quickly. The core lifecycle is in place, but the manifest and adapter contracts should still be treated as alpha.

Current areas of focus:

- state-driven teardown and recovery
- local and remote runners
- script, Docker Compose, Terraform, and CloudFormation components
- lifecycle hooks and component outputs
- state backend support for local files and S3
- safer handling of credentials, state, and sensitive outputs

## Quickstart

Build the CLI:

```sh
go build -o bin/orch ./cmd/orch
```

Create `orch.yaml`:

```yaml
version: orch.io/1.0

metadata:
  id: hello
  description: Local script example
  owner:
    name: Orch
    email: orch@example.com

runners:
  local:
    type: local
    config: {}

components:
  hello:
    type: script
    runner: local
    source:
      embedded: |
        echo "message=hello from orch" >> "$ORCH_OUTPUT_ENV"
    outputs:
      - name: message
    hooks:
      post_apply:
        - command: echo ${setup.outputs.message}
      post_destroy:
        - command: echo ${setup.outputs.message} and bye now!
```

Run it:

```sh
bin/orch up --env-id demo
bin/orch state inspect --env-id demo
bin/orch down --env-id demo
```

After a successful `down`, Orch deletes the environment state bundle.

## Documentation

The public documentation lives in `docs/` and is built with Astro Starlight.

```sh
cd docs
npm install
npm run dev
```

## Development

Run the Go test suite:

```sh
go test ./...
```

Smoke tests live in `tests/`. Some require local tools or cloud credentials and are opt-in.

## License

Orch is licensed under the terms in `LICENSE`.
