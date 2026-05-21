# Implementation Status

This document tracks temporary implementation state. The product spec in [SPEC.md](SPEC.md) describes the intended final behavior.

## Implemented

- Go CLI entrypoint for `terraform-registry`.
- Cobra command tree and grouped root help.
- Project commands:
  - `add <provider> [--version <constraint>]`
  - `remove <provider>`
  - `update <provider> --constraint <constraint>`
- Provider search through the Terraform Registry.
- Local `.tf` file edits using HashiCorp HCL tooling.
- Tests for CLI parsing, provider search, and project file edits.

## Temporary Limitations

- `docs` commands still print placeholder output.
- Project commands do not call the Terraform Registry.
- `add` only writes a `version` field when `--version` is provided.
- `update` currently requires `--constraint`; automatic latest-version resolution is not implemented yet.
