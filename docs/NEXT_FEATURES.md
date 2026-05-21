# Terraform Util Next Features Spec

## Overview

This document tracks planned behavior changes and new features for `terraform-util`.
It describes intended future behavior, not the current implementation.

## Goals

- Make detailed output explicit and easier to request.
- Fetch docs for the provider version used by the current Terraform project by default.
- Allow agents to intentionally request latest docs or a specific version.
- Expose provider version discovery through the CLI.
- Aggregate paginated registry responses so large provider result sets are complete by default.
- Add registry-only support for Terraform modules.

## CLI Changes

### Rename Detailed Output Flag

Replace the global detailed-output flag:

```sh
--verbose
```

with:

```sh
--details
-d
```

`--details` should include additional metadata useful to humans and agents. `--verbose`
should be removed unless a temporary compatibility window is explicitly chosen during
implementation.

### Docs Version Flags

Add docs version selection flags:

```sh
--version <version>
-v <version>
--latest
```

These flags apply to:

```sh
terraform-util docs list <provider> [keyword]
terraform-util docs <provider> <data|resource|function>/<name>
```

Version selection precedence:

1. `--latest` fetches docs for the latest registry version.
2. `--version` / `-v` fetches docs for the exact requested provider version.
3. If neither flag is provided, inspect the current directory for a matching provider in `terraform.required_providers` and use that version when it is an exact version.
4. If no matching required provider or exact version is found, fetch latest docs.

`--latest` and `--version` are mutually exclusive.

If the current project has a non-exact version constraint such as `~> 6.0`, `>= 5.0`, or
`>= 5.0, < 7.0`, the CLI should either resolve the newest matching version or clearly fall
back to latest. The preferred final behavior is to resolve the newest matching version.

## Search Output Changes

The `verified` column should display boolean-like values.

Current:

```text
provider       name  version  verified
hashicorp/aws  aws   6.46.0   verified
```

Future:

```text
provider       name  version  verified
hashicorp/aws  aws   6.46.0   true
```

For unverified providers, display an empty value.

When `--details` is set, search output should continue to include download counts also display tier.

## Provider Versions Command

Add a command that lists all known versions for a provider:

```sh
terraform-util versions <provider>
```

Example output:

```text
version
6.46.0
6.45.0
6.44.0
```

With details:

```sh
terraform-util --details versions aws
```

Example output:

```text
provider: registry.terraform.io/hashicorp/aws
website: https://registry.terraform.io/providers/hashicorp/aws

version  published
6.46.0   2026-05-20
6.45.0   2026-05-13
```

Version ordering should be newest first.

## Module Registry Support

Add support for Terraform Registry modules to registry commands only.

Affected commands:

```sh
terraform-util search <provider|module>
terraform-util docs <provider|module>
terraform-util versions <provider|module>
```

Project-editing commands remain provider-only:

```sh
terraform-util add <provider>
terraform-util update <provider>
terraform-util remove <provider>
```

### Address Forms

Provider inputs continue to support:

```text
aws
hashicorp/aws
registry.terraform.io/hashicorp/aws
```

Module inputs should support Terraform Registry module addresses:

```text
terraform-aws-modules/vpc/aws
registry.terraform.io/terraform-aws-modules/vpc/aws
```

Short module search should support plain text queries:

```sh
terraform-util search vpc
terraform-util search aws vpc
```

If a query could match both providers and modules, default output should clearly identify the
result type.

Example:

```text
type      source                         name  version  verified
provider  hashicorp/aws                  aws   6.46.0   true
module    terraform-aws-modules/vpc/aws  vpc   6.0.1    true
```

### Search

`search` should search both providers and modules by default:

```sh
terraform-util search vpc
```

Future optional flags may narrow the search:

```sh
--type provider
--type module
```

Do not add those narrowing flags unless implementation needs them.

### Module Docs

`docs` for modules should fetch module README-style documentation for a module version:

```sh
terraform-util docs terraform-aws-modules/vpc/aws
terraform-util docs registry.terraform.io/terraform-aws-modules/vpc/aws
```

Module docs do not use provider docs paths such as `resource/aws_vpc`.

Version selection for modules:

1. `--latest` fetches docs for the latest module version.
2. `--version` / `-v` fetches docs for the exact requested module version.
3. Default is latest.

Project `required_providers` version discovery does not apply to modules.

Detailed module docs output should include:

```text
Type: module
Module: registry.terraform.io/terraform-aws-modules/vpc/aws
Version: 6.0.1
Website: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/6.0.1
Source: https://github.com/terraform-aws-modules/terraform-aws-vpc
```

### Module Versions

`versions` should support module addresses:

```sh
terraform-util versions terraform-aws-modules/vpc/aws
```

Example output:

```text
version
6.0.1
6.0.0
5.21.0
```

Detailed output should identify the result as a module:

```text
type: module
module: registry.terraform.io/terraform-aws-modules/vpc/aws
website: https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws

version  published
6.0.1   2026-05-01
6.0.0   2026-04-20
```

## Detailed Docs Output

Detailed docs output should include both:

- Terraform Registry website URL.
- Source repository URL for the provider docs file, when available.

Example:

```text
Provider: registry.terraform.io/hashicorp/aws
Version: 6.46.0
Website: https://registry.terraform.io/providers/hashicorp/aws/6.46.0/docs/resources/vpc
Source: https://github.com/hashicorp/terraform-provider-aws/blob/v6.46.0/website/docs/r/vpc.html.markdown
Doc: resource/aws_vpc
```

For `docs list`, the website URL may point to the provider version docs root:

```text
Website: https://registry.terraform.io/providers/hashicorp/aws/6.46.0/docs
```

## Background Result Aggregation

Aggregate all registry pages in the background anywhere registry result sets can exceed the
API page size. Users should not need to request a second page to see the full result set.

Known affected commands:

```sh
terraform-util docs list <provider> [keyword]
terraform-util versions <provider>
terraform-util versions <module>
```

Default output should print every matching result. Detailed output may include the total
number of aggregated results:

```text
Total: 2485
```

If the Terraform Registry API does not expose total counts for a specific endpoint, the CLI
should continue fetching until the API indicates there are no more pages or until a returned
page contains fewer items than the requested page size.

No user-facing `--page` or `--page-size` flags should be added for this feature.

## Test Plan

- `--details` and `-d` parse globally.
- `--verbose` is rejected or handled according to the selected compatibility policy.
- Search verified column prints `true` for verified providers and `false` or empty for unverified providers.
- Docs commands honor version precedence: `--latest`, `--version`, exact project version, fallback latest.
- `--latest` and `--version` cannot be used together.
- Project provider version discovery reads `required_providers` from valid `.tf` files.
- `versions <provider>` resolves short, namespaced, and fully-qualified provider inputs.
- `versions <provider>` sorts newest first.
- `search` can return both provider and module results and clearly identifies each result type.
- Module addresses resolve in `docs` and `versions`.
- Module docs output fetches module README-style documentation without provider docs paths.
- Module docs version selection supports `--latest` and `--version`.
- Project provider version discovery does not affect module docs.
- Detailed module output includes module type, registry website URL, and source URL when available.
- Detailed docs output includes Terraform Registry website URL and source URL when source is available.
- Docs list displays all matching results beyond the first API page.
- `versions <provider>` displays all versions beyond the first API page.
- `versions <module>` displays all versions beyond the first API page.
- Registry client tests cover multi-page responses and termination at the final page.
- No user-facing paging flags are present in help output.
