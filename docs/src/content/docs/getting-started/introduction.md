---
title: Introduction
description: What Orch is and when to use it.
---

Orch is an environment orchestration tool for short-lived infrastructure and application sandboxes.

You define an environment in `orch.yaml`. Orch applies each component in dependency order, captures component outputs, persists operational state, and can later destroy the environment from that state.

## Good Fits

- Preview environments for pull requests
- Local or CI smoke environments
- Integration test sandboxes
- Temporary demos
- Multi-tool workflows that combine scripts, Compose, Terraform, or CloudFormation

## Current Shape

Orch is alpha software. The current implementation focuses on making the lifecycle reliable before broadening the ecosystem.

The project currently supports:

- local and SSH runners
- local and S3 state backends
- script, Docker Compose, Terraform, and CloudFormation adapters
- lifecycle hooks
- component outputs
- state inspection
- state-driven teardown
