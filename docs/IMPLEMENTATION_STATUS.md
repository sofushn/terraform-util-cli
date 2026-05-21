# Implementation Status

This document tracks temporary implementation state. The product spec in [SPEC.md](SPEC.md) describes the intended final behavior.

## Implemented

- Go CLI entrypoint for `terraform-registry`.
- Cobra command tree and grouped root help.
- Project commands:
  - `add <provider> [--version <constraint>]`
  - `remove <provider>`
  - `update <provider> [--version <constraint>]`
- Provider search through the Terraform Registry.
- Registry-backed provider resolution for `add` and `update`; short names choose the matching provider with the highest download count.
- `remove` stays local-only and does not verify against the registry.
- `add` and `update` use the latest provider version when no explicit version constraint is provided.
- Local `.tf` file edits using HashiCorp HCL tooling.
- Registry-backed `docs list <provider> [keyword]` and `docs <provider> <kind>/<name>` commands.
- Tests for CLI parsing, provider search, provider docs, and project file edits.

## Temporary Limitations

- None currently documented.
