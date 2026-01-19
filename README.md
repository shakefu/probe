# terraform-provider-probe

[![Tests](https://github.com/shakefu/terraform-provider-probe/actions/workflows/test.yml/badge.svg)](https://github.com/shakefu/terraform-provider-probe/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/shakefu/terraform-provider-probe/graph/badge.svg)](https://codecov.io/gh/shakefu/terraform-provider-probe)

A Terraform/OpenTofu provider that checks whether AWS resources exist
without failing when they don't.

Sponsored by [<img alt="Textla.com" width="72" height="29" alt="Textla Logo Go Green" src="https://github.com/user-attachments/assets/2856b6ea-2ec7-4c24-a454-855a826646a8" />](https://textla.com).

## The Problem

Terraform and OpenTofu have no native way to gracefully check if a resource
exists before deciding whether to create it. All existing approaches fail:

1. **Standard data sources** - Fail with an error if the resource doesn't exist
2. **`awscc` plural data sources** - Fail with "Empty result" when none exist
3. **`external` data source** - Requires bash scripts and manual configuration
4. **Import blocks** - Fail if the resource doesn't exist (no "optional import")

This provider solves that problem.

## Installation

```hcl
terraform {
  required_providers {
    probe = {
      source  = "shakefu/probe"
      version = "~> 0.1"
    }
  }
}

provider "probe" {}
```

## Usage

### Basic existence check

```hcl
data "probe" "my_table" {
  type = "aws_dynamodb_table"
  id   = "my-table"
}

output "table_exists" {
  value = data.probe.my_table.exists
}
```

### Create-or-adopt pattern

```hcl
data "probe" "contacts_table" {
  type = "aws_dynamodb_table"
  id   = "${var.prefix}-contacts"
}

resource "aws_dynamodb_table" "contacts" {
  count = data.probe.contacts_table.exists ? 0 : 1

  name         = "${var.prefix}-contacts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"

  attribute {
    name = "pk"
    type = "S"
  }
}
```

## Provider Configuration

```hcl
provider "probe" {
  # Optional: Explicitly enable/disable LocalStack detection
  # localstack = false

  # Optional: Override endpoint (implies localstack = true)
  # endpoint = "http://localhost:4566"

  # Optional: Explicit region (defaults to AWS_REGION, AWS_DEFAULT_REGION, then us-east-1)
  # region = "us-west-2"
}
```

The provider automatically detects LocalStack running at `localhost:4566`
and configures endpoints accordingly.

## Data Source: `probe`

### Arguments

- `type` (Required) - Resource type. Accepts Terraform-style names
  (`aws_dynamodb_table`) or AWS-style type names (`AWS::DynamoDB::Table`).
- `id` (Required) - Resource identifier (table name, bucket name, etc.).

### Attributes

- `exists` - Whether the resource exists.
- `arn` - Resource ARN (null if resource doesn't exist).
- `properties` - Resource properties as a map (null if resource doesn't exist).
  Includes resource-specific attributes and Tags when available.

## Supported Resource Types

The provider uses native AWS SDK calls for full property retrieval, including
Tags. Currently supported resource types:

| Terraform Type       | AWS Type               | Identifier     |
| -------------------- | ---------------------- | -------------- |
| `aws_dynamodb_table` | `AWS::DynamoDB::Table` | Table name     |
| `aws_s3_bucket`      | `AWS::S3::Bucket`      | Bucket name    |

Additional resource types will be added incrementally. Contributions welcome!

## Building from Source

```bash
go build -o terraform-provider-probe
```

## License

MPL-2.0
