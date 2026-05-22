# Implementation Status

This document tracks implementation state. The product spec in [SPEC.md](SPEC.md)
describes intended stable behavior, and [NEXT_FEATURES.md](NEXT_FEATURES.md)
tracks planned follow-up work.

## Implemented

- Go CLI entrypoint for `terraform-util`.
- Cobra command tree and grouped root help.
- Provider search through the official Terraform Registry.
- Search displays all provider search results returned by the registry across paged results, using registry pagination metadata when available.
- Search prints result pages progressively instead of waiting for every page to load.
- Stable-width search output with `true` in the `verified` column for verified providers.
- Detailed search output includes downloads and provider tier.
- Provider versions command:
  - `terraform-util versions <provider>`
  - `terraform-util --details versions <provider>`
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
- Docs version selection flags:
  - `--version <version>`
  - `-v <version>`
  - `--latest`
- Docs version default checks `.terraform.lock.hcl`, then `required_providers`, then latest registry docs.
- Non-exact `required_providers` constraints resolve to the newest matching registry version.
- `docs list` loads all provider docs pages for resources, data sources, and functions, using registry pagination metadata when available.
- `docs list` prints result pages progressively instead of waiting for every page to load.
- Detailed docs output includes:
  - provider source
  - provider version
  - Terraform Registry website URL
  - source repository URL when available
- Tests for CLI parsing, provider search, provider docs, and project file edits.

## Planned

- Registry-only module support for `search`, `docs`, and `versions`.
