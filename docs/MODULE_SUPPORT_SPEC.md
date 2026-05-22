# Module Registry Support Spec

## Summary

Add Terraform Registry module support to registry-oriented commands while keeping Terraform
project editing provider-only.

Affected registry commands:

```sh
terraform-util search <query>
terraform-util docs <provider> <data|resource|function>/<name>
terraform-util docs <module>
terraform-util versions <provider|module>
```

Unaffected project commands:

```sh
terraform-util add <provider>
terraform-util update <provider>
terraform-util remove <provider>
```

## Goals

- Let humans and LLM agents discover Terraform Registry modules from the same CLI surface as providers.
- Keep common commands short and infer provider vs module from address shape where possible.
- Avoid moving module concerns into `internal/project`.
- Preserve existing provider behavior and output compatibility as much as possible.

## Non-Goals

- Do not add module support to `add`, `update`, or `remove`.
- Do not edit Terraform `module` blocks.
- Do not support private registries.
- Do not require users to pass `--module` for normal module commands.

## Terminal UX

Prefer address-shape inference over required flags.

Provider examples:

```sh
terraform-util search aws
terraform-util docs aws resource/aws_vpc
terraform-util docs hashicorp/aws data/aws_ami
terraform-util versions hashicorp/aws
```

Module examples:

```sh
terraform-util search vpc --type module
terraform-util docs terraform-aws-modules/vpc/aws
terraform-util versions terraform-aws-modules/vpc/aws
```

Search supports type selection:

```sh
terraform-util search aws
terraform-util search vpc --type module
terraform-util search vpc -t module
terraform-util search vpc -m
terraform-util search aws --type provider
terraform-util search aws -p
terraform-util search vpc --type all
```

`--type` shorthand is `-t`.
`-m` is shorthand for module-only search.
`-p` is shorthand for provider-only search.

Default search type is `provider`.

Search query shape:

- `search` accepts exactly one query argument.
- Multi-argument search such as `terraform-util search aws vpc -m` is not supported.
- Quoted multi-word search such as `terraform-util search "aws vpc" -m` is not supported.
- Search queries should be single tokens such as `aws`, `vpc`, or `terraform-aws-modules`.

Search type flag rules:

- `--type provider`, `-t provider`, and `-p` are equivalent.
- `--type module`, `-t module`, and `-m` are equivalent.
- `--type all` has no dedicated shorthand beyond `-t all`.
- Type selectors are mutually exclusive. For example, `-m -p` and `-m --type all` should fail.

## Address Forms

Provider inputs continue to support:

```text
aws
hashicorp/aws
registry.terraform.io/hashicorp/aws
```

Module inputs should support:

```text
terraform-aws-modules/vpc/aws
registry.terraform.io/terraform-aws-modules/vpc/aws
```

Inference rules:

- `docs list <provider> [keyword]` is provider docs list.
- `docs <provider> <docs-path>` is provider docs.
- `docs <module>` is module docs.
- `versions <three-part-address>` is module versions.
- `versions <one-or-two-part-address>` is provider versions.
- Fully-qualified `registry.terraform.io/...` addresses should be normalized before inference.

Short names:

- A one-part address such as `aws` is provider-first for exact commands.
- `docs aws resource/aws_vpc` means provider docs for `aws`.
- `versions aws` means provider versions for `aws`.
- Module docs and module versions require a module-shaped address such as
  `terraform-aws-modules/vpc/aws`.
- `docs aws` should not guess a module. It should fail with a targeted message because provider
  docs require a docs path and module docs require a module address.
- `search aws` defaults to provider search only.
- `search aws --type all` may return both provider and module results and should include `type`.

Rationale: Registry modules are not safely identified by one segment. A short name like `aws`
could refer to many modules, while current provider behavior already treats it as a provider
lookup.

## Search

Default search should return providers only:

```sh
terraform-util search aws
```

Default provider-only output should not include a `type` column. It should keep the current
provider search table shape.

Module search:

```sh
terraform-util search vpc --type module
terraform-util search vpc -t module
terraform-util search vpc -m
```

Module-only output should not include a `type` column because the result type is known from the
flag.

Mixed search:

```sh
terraform-util search vpc --type all
```

Only mixed `--type all` output should include a `type` column:

```text
type      source                         name  version  verified
provider  hashicorp/aws                  aws   6.46.0   true
module    terraform-aws-modules/vpc/aws  vpc   6.0.1    true
```

Detailed output should continue to include provider-specific fields where available, and module
specific fields where available.

Invalid search examples:

```sh
terraform-util search aws vpc -m
terraform-util search "aws vpc" -m
```

Both should fail with a clear message that search accepts one single-token query.

## Module Docs

Module docs should fetch README-style documentation for a module version:

```sh
terraform-util docs terraform-aws-modules/vpc/aws
terraform-util docs registry.terraform.io/terraform-aws-modules/vpc/aws
```

`docs` should support both provider docs and module docs by arity and address shape:

```sh
terraform-util docs <module-address>
terraform-util docs <provider-address> <data|resource|function>/<name>
```

Parser behavior:

- `docs list <provider> [keyword]` is always provider docs list.
- `docs <three-part-module-address>` fetches module docs.
- `docs <provider-address> <docs-path>` fetches provider docs.
- `docs <provider-address>` with a one- or two-part provider address fails with a targeted error.
- `docs <module-address> <docs-path>` fails because module docs are module-level.

Module docs do not accept provider docs paths:

```sh
terraform-util docs terraform-aws-modules/vpc/aws resource/aws_vpc
```

That command should fail with a clear error because module docs are fetched at module level.

Module docs content:

- Default module docs output is the module README body only.
- Do not include module inputs, outputs, dependencies, examples, or other structured metadata in
  the first implementation.
- `--details` may prepend module metadata, but the documentation body remains README-only.

Version selection:

1. `--latest` fetches latest module docs.
2. `--version` / `-v` fetches exact module docs version.
3. Default is latest.

Project lock-file and `required_providers` version discovery does not apply to modules.

Detailed module docs output:

```text
Type: module
Module: registry.terraform.io/terraform-aws-modules/vpc/aws
Version: 6.0.1
Website: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/6.0.1
Source: https://github.com/terraform-aws-modules/terraform-aws-vpc

# Terraform AWS VPC
...
```

## Module Versions

`versions` should support module addresses:

```sh
terraform-util versions terraform-aws-modules/vpc/aws
```

Default output:

```text
version
6.0.1
6.0.0
5.21.0
```

Detailed output:

```text
type: module
module: registry.terraform.io/terraform-aws-modules/vpc/aws
website: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws

version  published
6.0.1   2026-05-01
6.0.0   2026-04-20
```

Version ordering should be newest first.

## Architecture

Keep package responsibilities narrow:

- `internal/cli`: command shape, flags, output formatting.
- `internal/app`: resolves registry object type and coordinates use cases.
- `internal/registry`: official Registry provider and module API clients.
- `internal/project`: provider-only local Terraform project inspection/editing.

Suggested app-level model:

```go
type RegistryObjectType string

const (
    RegistryObjectProvider RegistryObjectType = "provider"
    RegistryObjectModule   RegistryObjectType = "module"
)

type RegistryObject struct {
    Type          RegistryObjectType
    Source        string
    Name          string
    LatestVersion string
    Downloads     int64
    Verified      bool
}
```

Provider-specific and module-specific API payload structs should stay in `internal/registry`.

## Compatibility

- Existing provider docs commands must continue working.
- Existing project commands remain provider-only.
- Existing docs version flags remain valid.
- `--latest` and `--version` remain mutually exclusive for docs.

## Test Plan

- Search defaults to provider-only results.
- Search supports `--type provider`, `--type module`, `--type all`, and `-t`.
- Search supports `-p` for provider-only and `-m` for module-only.
- Search rejects multi-argument and quoted multi-word queries.
- Conflicting search type selectors fail.
- Search only displays the `type` column for `--type all`.
- Provider-only and module-only search output do not display `type`.
- Provider docs path commands still work.
- Module docs command fetches module README-style docs.
- Module docs rejects provider docs paths.
- Module docs `--version`, `-v`, and `--latest` work.
- Module docs default version is latest.
- Provider docs project version detection does not affect module docs.
- Provider versions still work.
- Module versions list newest first.
- Module versions detailed output includes module URL.
- Project commands reject or ignore module addresses as they do today for invalid provider shapes.

## Open Questions

No open questions remain in the current module-support design.
