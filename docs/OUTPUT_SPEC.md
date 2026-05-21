# Terraform Registry CLI Output Spec

## Overview

This document defines optional structured output formats for `terraform-registry`.

The main implementation spec keeps the default CLI output plain and human-readable. This spec captures richer JSON and Markdown-style contracts for future agent integrations, tests, and compatibility guarantees.

## Format Option

A future implementation may support:

```sh
--format <text|json|markdown>
```

Recommended behavior:

- `text`: default human-readable output.
- `json`: stable machine-readable output for search, list, mutations, and errors.
- `markdown`: documentation-oriented output for fetched docs pages.

## Search JSON

Command:

```sh
terraform-registry search aws --format json
```

Output:

```json
{
  "query": "aws",
  "providers": [
    {
      "source": "registry.terraform.io/hashicorp/aws",
      "namespace": "hashicorp",
      "name": "aws",
      "display_name": "AWS",
      "description": "Manage AWS resources.",
      "latest_version": "6.0.0",
      "downloads": 123456789,
      "verified": true
    }
  ]
}
```

## Docs List JSON

Command:

```sh
terraform-registry docs list aws vpc --format json
```

Output:

```json
{
  "provider": "registry.terraform.io/hashicorp/aws",
  "version": "6.0.0",
  "keyword": "vpc",
  "items": [
    {
      "type": "resource",
      "name": "aws_vpc",
      "title": "aws_vpc",
      "path": "resource/aws_vpc"
    },
    {
      "type": "data",
      "name": "aws_vpc",
      "title": "aws_vpc",
      "path": "data/aws_vpc"
    }
  ]
}
```

## Docs Markdown

Command:

```sh
terraform-registry docs hashicorp/aws resource/aws_vpc --format markdown
```

Output:

```markdown
---
provider: registry.terraform.io/hashicorp/aws
version: 6.0.0
type: resource
name: aws_vpc
source_url: https://registry.terraform.io/providers/hashicorp/aws/6.0.0/docs/resources/vpc
fetched_at: 2026-05-21T13:00:00Z
---

# aws_vpc

...
```

Every fetched docs page should include:

- Provider source.
- Provider version.
- Documentation type.
- Documentation name.
- Source URL.
- Fetch timestamp.

## Mutation JSON

Commands such as `add`, `remove`, and `update` should return a small summary when `--format json` is used.

Example:

```json
{
  "ok": true,
  "action": "add",
  "provider": "registry.terraform.io/hashicorp/aws",
  "version_constraint": "~> 6.0",
  "changed_files": [
    "versions.tf",
    "providers.tf"
  ]
}
```

## Error JSON

JSON errors should be stable and include machine-readable error details:

```json
{
  "ok": false,
  "error": {
    "code": "provider_not_found",
    "message": "No provider matched \"awss\".",
    "suggestions": ["hashicorp/aws"]
  }
}
```

Recommended error fields:

- `code`: stable snake_case machine-readable code.
- `message`: concise human-readable message.
- `suggestions`: optional list of possible fixes or provider matches.
- `details`: optional object with command-specific context.
