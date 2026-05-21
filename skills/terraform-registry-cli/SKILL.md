---
name: terraform-registry-cli
description: Use when an agent needs current Terraform Registry provider information or docs through the local terraform-registry CLI, or needs to add, update, or remove providers in the current Terraform project.
---

# Terraform Registry CLI

Use `terraform-registry` to search the official Terraform Registry, fetch current provider docs, and edit provider requirements in local `.tf` files.

The CLI is designed for agents: prefer it over relying on stale Terraform provider knowledge.

## Prerequisites

From this repository, build or install the CLI when it is not already on `PATH`:

```sh
go build -o terraform-registry ./cmd/terraform-registry
./terraform-registry --help
```

Or install it into the Go bin directory:

```sh
go install ./cmd/terraform-registry
terraform-registry --help
```

If running from the repo without installing, use `./terraform-registry`. Otherwise use `terraform-registry`.

## Command Tree

```text
terraform-registry
|-- search <provider>
|-- docs
|   |-- list <provider> [keyword]
|   `-- <provider> <data/name|resource/name|function/name>
|-- add <provider> [--version <constraint>]
|-- update <provider> [--version <constraint>]
`-- remove <provider>
```

Global flags:

```sh
--verbose   # include extra metadata
--quiet     # suppress non-essential output
```

There is no custom registry flag; the CLI uses the official Terraform Registry.

## Provider Inputs

Provider arguments may be:

```text
aws
hashicorp/aws
registry.terraform.io/hashicorp/aws
```

For short names, the CLI resolves against the official registry. If multiple providers share the same name, `add` and `update` use the exact name with the highest download count.

## Search Providers

Use search before choosing a provider:

```sh
terraform-registry search aws
terraform-registry --verbose search aws
```

Default output columns are stable-width:

```text
provider       name  version  verified
hashicorp/aws  aws   6.46.0   verified
```

Verbose search also includes downloads:

```text
provider       name  version  downloads  verified
hashicorp/aws  aws   6.46.0   6254226571 verified
```

Only verified providers display `verified`; unverified providers have an empty value in that column.

## Read Provider Docs

List available resources, data sources, and functions:

```sh
terraform-registry docs list aws
terraform-registry docs list aws vpc
terraform-registry --verbose docs list aws vpc
```

Default list output is one docs path per line:

```text
resource/aws_vpc
data/aws_vpc
function/arn_parse
```

Fetch a specific docs page:

```sh
terraform-registry docs aws resource/aws_vpc
terraform-registry docs hashicorp/aws data/aws_ami
terraform-registry docs aws function/arn_parse
```

Default docs output is the markdown-like documentation body only. Use `--verbose` when you need provider, version, doc path, and source URL metadata:

```sh
terraform-registry --verbose docs aws resource/aws_vpc
```

Agent pattern:

```sh
terraform-registry docs list aws vpc
terraform-registry docs aws resource/aws_vpc
```

Use `docs list` first when you are unsure of the exact docs path. Resource and data source docs usually use Terraform type names such as `resource/aws_vpc` and `data/aws_ami`.

## Edit Terraform Projects

Project commands operate on `.tf` files in the current working directory. Run them from the Terraform project directory and inspect the diff afterward.

Add a provider:

```sh
terraform-registry add aws
terraform-registry add aws --version "~> 6.0"
```

When `--version` is omitted, the CLI resolves and writes the latest provider version. `add` verifies the provider against the registry, updates `required_providers`, and creates a basic `provider "<name>" {}` block if needed.

Update a provider version:

```sh
terraform-registry update aws
terraform-registry update aws --version "~> 6.1"
```

When `--version` is omitted, the CLI resolves and writes the latest provider version. `update` verifies the provider against the registry and requires the provider to already exist in `required_providers`.

Remove a provider:

```sh
terraform-registry remove aws
```

`remove` is intentionally local-only and does not verify against the registry. This allows removing stale or unpublished provider entries.

Safety checklist for agents:

```sh
terraform-registry add aws
git diff -- '*.tf'
go test ./...
```

Do not run project commands from a parent directory unless the user explicitly wants to edit provider declarations there.

## Error Handling

Invalid command shapes print help and return a nonzero exit code. If a docs path fails, list docs with a keyword and retry with the exact path:

```sh
terraform-registry docs list aws ami
terraform-registry docs aws data/aws_ami
```

Use `--quiet` only when another workflow already confirms success through exit codes or file diffs.
