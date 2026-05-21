# Terraform Registry CLI Implementation Spec

## Overview

`terraform-registry` is a command-line tool for searching Terraform Registry providers, listing their available documentation, fetching current provider docs, and safely editing Terraform configuration files to add, remove, or update provider requirements.

The primary user is an LLM agent that needs reliable, current Terraform documentation without relying on stale training data. Human users should also be able to use the CLI directly.

## Goals

- Provide a small CLI surface for provider discovery, documentation lookup, and Terraform provider management.
- Fetch documentation from the live Terraform Registry or its backing APIs.
- Return predictable plain text output suitable for humans and LLM agents.
- Modify Terraform files safely and idempotently.
- Preserve existing Terraform formatting and user-authored configuration as much as possible.

## Non-Goals

- Replace `terraform init`, `terraform providers`, or Terraform's dependency lock behavior.
- Generate complete Terraform configurations from scratch.
- Support every Terraform Registry feature in the first version.
- Edit remote modules or files outside the current working tree.

## CLI Shape

The executable name should be:

```sh
terraform-registry
```

### Commands

```sh
terraform-registry search <provider>
terraform-registry add <provider>
terraform-registry remove <provider>
terraform-registry update <provider>
terraform-registry docs list <provider> [keyword]
terraform-registry docs <provider> <data|resource|function>/<name>
```

### Global Options

Recommended global options:

```sh
--verbose
--quiet
```

The default output should be plain text with basic, useful information. Detailed machine-readable output contracts can be specified separately; see [OUTPUT_SPEC.md](OUTPUT_SPEC.md).

## Provider Address Handling

Provider inputs should accept:

- Short provider names, such as `aws`.
- Namespace-qualified provider names, such as `hashicorp/aws`.
- Fully-qualified registry source addresses, such as `registry.terraform.io/hashicorp/aws`.

Internally normalize all provider references into:

```text
<hostname>/<namespace>/<name>
```

For public registry providers, the default hostname is:

```text
registry.terraform.io
```

If the namespace is omitted, the CLI should search the official Terraform Registry and select the matching provider name with the highest download count. For common official providers this usually resolves to `hashicorp/<name>`, but the CLI should confirm against registry data instead of hardcoding.

## Command Behavior

### `search <provider>`

Search the Terraform Registry for matching providers.

Example:

```sh
terraform-registry search aws
```

Example output:

```text
hashicorp/aws  AWS  6.0.0  verified
```

The default search result should include enough information to choose a provider:

```text
<namespace>/<name>  <display name>  <version>  [verified]
```

Only display `verified` for verified providers. Do not display an unverified status label by default.

With `--verbose`, search output should also include download counts:

```text
hashicorp/aws  AWS  6.0.0  verified  downloads: 123456789
```

Selection ranking should prefer:

1. Exact source match.
2. Exact namespace/name match.
3. Verified providers.
4. Download count.
5. Text relevance.

### `add <provider>`

Add a provider requirement and provider block to valid Terraform files in the current directory.

Recommended behavior:

- Resolve and verify the provider against the official Terraform Registry before editing files.
- Discover `.tf` files in the current directory.
- Parse HCL instead of using ad hoc string replacement.
- Find or create a `terraform` block.
- Find or create `required_providers`.
- Add the provider under its local name.
- Add a basic `provider "<name>" {}` block if no provider block exists.
- Preserve existing provider configuration if present.
- Do not edit generated or vendored directories such as `.terraform`.

Example resulting HCL:

```hcl
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {}
```

Version constraint policy:

- Resolve the latest provider version from the registry.
- Default to the latest provider version when `--version` is omitted.
- Allow an override with `--version`.

If multiple `.tf` files exist, the CLI should prefer:

1. A file already containing a `terraform` block.
2. `versions.tf`.
3. `providers.tf`.
4. Create `versions.tf`.

The provider block should prefer:

1. A file already containing provider blocks.
2. `providers.tf`.
3. Create `providers.tf`.

The command must be idempotent. Running it twice should not duplicate blocks or entries.

### `remove <provider>`

Remove the provider requirement and empty provider block for the provider.

Recommended behavior:

- Do not verify against the registry; removal should work for stale, unpublished, or locally misconfigured providers.
- Remove the provider entry from `terraform.required_providers`.
- Remove only provider blocks that are safe to delete.
- A provider block is safe to delete if it is empty or was previously generated by this CLI.
- Do not remove configured provider blocks with arguments, aliases, or comments unless explicitly supported by a future `--force` option.
- Leave unrelated providers untouched.

If removing the provider would leave an empty `required_providers` block, the CLI may remove the empty block. If this leaves an empty `terraform` block, it may remove that too.

### `update <provider>`

Update the provider version constraint in `required_providers`.

Recommended behavior:

- Resolve and verify the provider against the official Terraform Registry before editing files.
- Resolve the latest version from the registry.
- Update only the selected provider's `version` field.
- Default to the latest provider version when `--version` is omitted.
- Preserve the existing constraint style where possible:
  - `~> 5.0` becomes `~> 6.0`.
  - `>= 5.0` becomes `>= 6.0.0`.
  - Exact version `5.42.0` becomes the latest exact version.
- If the provider is not present, return a clear error suggesting `add`.

Future option:

```sh
terraform-registry update aws --version "~> 6.0"
```

### `docs list <provider> [keyword]`

List available resources, data sources, and functions for a provider.

Example:

```sh
terraform-registry docs list aws vpc
```

Example output:

```text
resource/aws_vpc
data/aws_vpc
```

With `--verbose`, include provider metadata before the list:

```text
Provider: registry.terraform.io/hashicorp/aws
Version: 6.0.0

resource/aws_vpc
data/aws_vpc
```

Keyword matching should search names, titles, and descriptions where available.

### `docs <provider> <data|resource|function>/<name>`

Fetch the documentation page for a specific item.

Examples:

```sh
terraform-registry docs aws resource/aws_vpc
terraform-registry docs hashicorp/aws data/aws_ami
```

Expected behavior:

- Resolve the provider and latest version unless the project pins a version.
- Fetch the matching documentation page.
- Emit the documentation as plain text or Markdown-like text that is readable in a terminal.
- Do not include provider metadata in default output.
- Include provider metadata when `--verbose` is set.

Example output:

```text
# aws_vpc

...
```

Verbose example output:

```text
Provider: registry.terraform.io/hashicorp/aws
Version: 6.0.0
Doc: resource/aws_vpc
Source: https://registry.terraform.io/providers/hashicorp/aws/6.0.0/docs/resources/vpc

# aws_vpc

...
```

For LLM usage, agents that need citation metadata should pass `--verbose` so the output contains the provider version and source URL.

## Registry Integration

The implementation should use Terraform Registry APIs where available. The public registry base URL is:

```text
https://registry.terraform.io
```

Recommended client responsibilities:

- Search providers.
- Resolve provider source addresses.
- Fetch latest provider version.
- Fetch documentation index for a provider version.
- Fetch individual documentation pages.

The client should be isolated behind an interface so tests can use fixtures without live network access.

## Documentation Fetching Strategy

Terraform Registry documentation may be available through API endpoints or rendered registry pages. Prefer structured API responses when available. If scraping rendered pages is necessary, isolate that logic and test it against fixtures.

The default docs output should be readable plain text with lightweight Markdown-style headings where useful. If the upstream response is HTML, convert it to clean text and strip navigation chrome.

Required metadata for every fetched docs page:

- Provider source.
- Provider version.
- Documentation type.
- Documentation name.
- Source URL.
- Fetch timestamp.

## Terraform File Editing

Use a real HCL parser and writer.

Recommended properties:

- Parse all candidate `.tf` files before modifying anything.
- Fail before writing if any Terraform file has invalid syntax.
- Preserve comments and formatting when possible.
- Write changes atomically.
- Produce a summary of changed files.

The editor should model Terraform configuration as:

```text
Project
  files
  terraform blocks
  required_providers entries
  provider blocks
```

Important edge cases:

- Existing `required_providers` without the target provider.
- Existing provider requirement without a version.
- Existing provider requirement with a different source.
- Existing aliased provider blocks.
- Multiple Terraform blocks across files.
- Empty directory with no `.tf` files.
- Invalid HCL in one or more files.
- Provider local name differing from provider source name.

Conflicting provider source behavior:

- If local name `aws` already points to a different source, do not overwrite silently.
- Return an actionable error and require a future explicit option such as `--replace`.

## Output and Exit Codes

### Exit Codes

```text
0  success
1  general error
2  invalid arguments
3  provider not found
4  docs item not found
5  invalid Terraform configuration
6  conflicting Terraform configuration
7  network or registry error
```

### Plain Text Output

Plain text output should be concise and command-oriented:

```text
Added provider registry.terraform.io/hashicorp/aws (~> 6.0)
Changed:
  versions.tf
  providers.tf
```

Errors should also be plain text by default:

```text
Error: no provider matched "awss"
Suggestion: hashicorp/aws
```

Machine-readable output can be added later or specified in [OUTPUT_SPEC.md](OUTPUT_SPEC.md).

## Suggested Internal Architecture

```text
src/
  cli/
    args
    commands
    output
  registry/
    client
    provider_resolution
    docs
  terraform/
    discovery
    parser
    editor
    writer
  errors
  main
```

Core interfaces:

```text
RegistryClient
  searchProviders(query)
  getProviderVersions(source)
  getLatestVersion(source)
  listDocs(source, version)
  getDoc(source, version, kind, name)

TerraformProjectEditor
  load(cwd)
  addProvider(source, versionConstraint)
  removeProvider(source)
  updateProvider(source, versionConstraint)
  write()
```

## Language and Library Notes

Good implementation choices:

- Go, because Terraform itself and the HCL libraries are Go-native.
- Rust, if using mature HCL parsing/editing crates and prioritizing single-binary distribution.
- Python, if fast prototyping matters more than formatting-preserving HCL edits.

Recommended first implementation language: Go.

Useful Go packages:

- `github.com/hashicorp/hcl/v2`
- `github.com/hashicorp/hcl/v2/hclwrite`
- `github.com/zclconf/go-cty/cty`
- `github.com/spf13/cobra`

## Testing Strategy

### Unit Tests

- Provider source normalization.
- Search result ranking.
- Version constraint generation.
- Docs index filtering.
- Error mapping and plain text output.

### Fixture Tests

- Registry responses from local JSON/HTML fixtures.
- Docs conversion fixtures.
- Terraform editing fixtures with before/after `.tf` files.

### Integration Tests

- Live registry smoke tests behind an opt-in flag.
- CLI command tests using temporary directories.

### Idempotency Tests

For `add`, `remove`, and `update`:

1. Run command once.
2. Capture output files.
3. Run command again.
4. Assert no unexpected file changes.

## MVP Scope

The first usable version should include:

- `search <provider>`
- `docs list <provider> [keyword]`
- `docs <provider> resource/<name>`
- `docs <provider> data/<name>`
- `add <provider>`
- Plain text output
- HCL-based Terraform edits

The first version may defer:

- Provider functions docs, if registry support is inconsistent.
- `remove`.
- `update`.
- Advanced version constraint flags.

## Future Enhancements

- `--version` for docs and provider edits.
- `--provider-file` and `--versions-file` options.
- OpenTofu registry support.
- Shell completions.
- Provider docs full-text search.
- Local vector or SQLite index for docs.
- MCP server mode for direct agent integration.
- `terraform-registry explain <address>` to fetch docs for Terraform addresses like `aws_vpc.main`.
