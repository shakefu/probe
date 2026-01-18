---
page_title: "Provider: Probe"
subcategory: ""
description: |-
  Check whether AWS resources exist without failing when they don't.
---

# Probe Provider

The Probe provider checks whether AWS resources exist without failing when they
don't. This solves a common Terraform limitation where data sources fail with
an error if the resource doesn't exist.

Use this provider to implement create-or-adopt patterns, conditional resource
creation, and safe deployments in environments where resources may or may not
already exist.

## Use Cases

- **Create-or-adopt pattern**: Create a resource if it doesn't exist,
  reference it if it does
- **E2E testing**: Run tests against LocalStack with or without pre-existing
  resources
- **Multi-tool environments**: Terraform configs that coexist with CDK,
  CloudFormation, or manual resources
- **Safe deployments**: Avoid destroying resources created by other tools

## How It Works

The provider uses native AWS SDK calls to check for resource existence.
When a resource doesn't exist, it returns `exists = false` instead of failing
with an error. Properties and Tags are retrieved when the resource exists.

## Example Usage

```terraform
terraform {
  required_providers {
    probe = {
      source = "shakefu/probe"
    }
    aws = {
      source = "hashicorp/aws"
    }
  }
}

provider "probe" {}

provider "aws" {
  region = "us-east-1"
}

data "probe" "my_table" {
  type = "aws_dynamodb_table"
  id   = "my-table"
}

resource "aws_dynamodb_table" "my_table" {
  count = data.probe.my_table.exists ? 0 : 1

  name         = "my-table"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "pk"

  attribute {
    name = "pk"
    type = "S"
  }
}
```

## Provider Configuration

```terraform
provider "probe" {
  # Optional: Explicit region (defaults to AWS_REGION, AWS_DEFAULT_REGION, then us-east-1)
  # region = "us-west-2"

  # Optional: Override endpoint for LocalStack or other compatible services
  # endpoint = "http://localhost:4566"

  # Optional: Explicitly enable/disable LocalStack detection (auto-detects by default)
  # localstack = true
}
```

### LocalStack Support

The provider automatically detects LocalStack running at `localhost:4566` and
configures itself accordingly. No additional configuration is needed for local
development with LocalStack.

### Authentication

The provider uses the standard AWS credential chain:

1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (EC2, ECS, Lambda)

For LocalStack, dummy credentials are automatically used if none are
configured.

## Schema

### Optional

- `region` (String) AWS region. Defaults to `AWS_REGION` environment variable,
  then `AWS_DEFAULT_REGION`, then `us-east-1`.
- `endpoint` (String) Custom endpoint URL for AWS APIs. Useful for LocalStack
  or other compatible services. Setting this implies `localstack = true`.
- `localstack` (Boolean) Explicitly enable or disable LocalStack detection.
  If not set, auto-detects LocalStack at `localhost:4566`.

## Supported Resource Types

| Terraform Type       | AWS Type               | Identifier     |
| -------------------- | ---------------------- | -------------- |
| `aws_dynamodb_table` | `AWS::DynamoDB::Table` | Table name     |
| `aws_s3_bucket`      | `AWS::S3::Bucket`      | Bucket name    |

Additional resource types will be added incrementally.
