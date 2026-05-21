# Implementation Status

This document tracks implementation state. The product spec in [SPEC.md](SPEC.md)
describes intended stable behavior, and [NEXT_FEATURES.md](NEXT_FEATURES.md)
tracks planned follow-up work.

## Implemented

- Go CLI entrypoint for `terraform-util`.
- Cobra command tree and grouped root help.
- Provider search through the official Terraform Registry.
- Stable-width search output with `true` in the `verified` column for verified providers.
- Detailed output flag:
  - `--details`
  - `-d`
- Project commands:
  - `add <provider> [--version <constraint>]`
  - `remove <provider>`
  - `update <provider> [--version <constraint>]`
- Registry-backed provider resolution for `add` and `update`; short names choose the matching provider with the highest download count.
- `remove` stays local-only and does not verify against the registry.
- `add` and `update` use the latest provider version when no explicit version constraint is provided.
- Local `.tf` file edits using HashiCorp HCL tooling.
- Registry-backed `docs list <provider> [keyword]` and `docs <provider> <kind>/<name>` commands.
- `docs list` aggregates all provider docs pages in the background for resources, data sources, and functions.
- Detailed docs output includes:
  - provider source
  - provider version
  - Terraform Registry website URL
  - source repository URL when available
- Tests for CLI parsing, provider search, provider docs, and project file edits.

## Planned

- Docs version selection:
  - `--version <version>`
  - `-v <version>`
  - `--latest`
- Default docs version selection from matching `required_providers` entries in the current Terraform project.
- Provider versions command:
  - `terraform-util versions <provider>`
- Registry-only module support for `search`, `docs`, and `versions`.
