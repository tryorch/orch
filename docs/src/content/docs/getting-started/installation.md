---
title: Installation
description: Build and run the Orch CLI.
---

Orch does not have published release artifacts yet. Build it from source.

## Requirements

- Go
- Git
- Any tools required by the adapters you use, such as Docker, Terraform, or AWS CLI

## Build

From the repository root:

```sh
go build -o bin/orch ./cmd/orch
```

Check the CLI:

```sh
bin/orch version
```

## Documentation Site

The docs site is a separate Starlight app:

```sh
cd docs
npm install
npm run dev
```
