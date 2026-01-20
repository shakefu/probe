# terraform-provider-probe

[![Tests](https://github.com/shakefu/terraform-provider-probe/actions/workflows/test.yml/badge.svg)](https://github.com/shakefu/terraform-provider-probe/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/shakefu/terraform-provider-probe/graph/badge.svg)](https://codecov.io/gh/shakefu/terraform-provider-probe)

A Terraform/OpenTofu provider that checks whether AWS resources exist
without failing when they don't.

<!-- markdownlint-disable MD013 MD033 -->
Sponsored by [<img alt="Textla.com" width="72" height="29" src="https://github.com/user-attachments/assets/2856b6ea-2ec7-4c24-a454-855a826646a8" />](https://textla.com).
<!-- markdownlint-enable MD013 MD033 -->

Terraform and OpenTofu have no native way to gracefully check if a resource
exists before deciding whether to create it.

The current ways to find and read existing resources for shared configuration
all either fail, or have problematic external dependencies:

- Standard data sources - Fail with an error if the resource doesn't exist
- `awscc` plural data sources - Fail with "Empty result" when none exist
- `external` data source - Requires bash scripts and manual configuration
<!-- markdownlint-disable MD013 -->
- [`import` blocks](https://developer.hashicorp.com/terraform/language/import) - Fail if the resource doesn't exist (no "optional import")
<!-- markdownlint-enable MD013 -->

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

## Data Source: `probe_iam_policy_simulation`

Probes IAM permissions using the AWS Policy Simulator API without failing
when permissions are denied. This follows the probe provider philosophy of
returning results instead of errors.

### IAM Simulation Arguments

- `policy_source_arn` (Required) - ARN of the IAM principal (user, role, group)
  whose policies will be tested.
- `actions` (Required) - List of IAM actions to simulate (e.g., `s3:GetObject`,
  `iam:CreateUser`).
- `resource_arns` (Optional) - List of resource ARNs to simulate against.
  Defaults to `["*"]` if not specified.
- `caller_arn` (Optional) - ARN of user to simulate as caller. Defaults to
  `policy_source_arn` if it's a user.
- `additional_policies` (Optional) - Additional IAM policies (as JSON strings)
  to include in simulation.
- `permissions_boundary_policies` (Optional) - Permissions boundary policies
  (as JSON strings) to apply.
- `resource_policy` (Optional) - Resource-based policy (as JSON string) for
  simulation.
- `context` (Optional) - Context entries for condition evaluation. Each entry
  has:
  - `key` - Context key name (e.g., `aws:CurrentTime`)
  - `type` - Value type: `string`, `stringList`, `numeric`, `numericList`,
    `boolean`, `booleanList`, `date`, `dateList`, `ip`, `ipList`, `binary`,
    `binaryList`
  - `values` - List of values for the context key

### IAM Simulation Attributes

- `allowed` - `true` if ALL actions are allowed for ALL resources. This is the
  primary output for simple permission checks.
- `error` - Error message if simulation could not be performed (e.g., principal
  doesn't exist). Null if successful.
- `results` - Detailed results for each action/resource combination:
  - `action` - The action that was evaluated
  - `resource_arn` - The resource ARN evaluated
  - `allowed` - Whether this specific action/resource is allowed
  - `decision` - `allowed`, `explicitDeny`, or `implicitDeny`
  - `matched_statements` - Statements that contributed to the decision
  - `missing_context_keys` - Context keys required but not provided

### Example: Simple Permission Check

```hcl
data "probe_iam_policy_simulation" "s3_access" {
  policy_source_arn = aws_iam_role.my_role.arn
  actions           = ["s3:GetObject", "s3:PutObject"]
  resource_arns     = ["arn:aws:s3:::my-bucket/*"]
}

output "can_access_s3" {
  value = data.probe_iam_policy_simulation.s3_access.allowed
}
```

### Example: Conditional Resource Creation

```hcl
data "probe_iam_policy_simulation" "lambda_permissions" {
  policy_source_arn = aws_iam_role.lambda_role.arn
  actions           = ["dynamodb:GetItem", "dynamodb:PutItem"]
  resource_arns     = [aws_dynamodb_table.my_table.arn]
}

# Only create additional permissions if the role doesn't already have them
resource "aws_iam_role_policy" "additional_permissions" {
  count = data.probe_iam_policy_simulation.lambda_permissions.allowed ? 0 : 1

  name = "additional-dynamodb-access"
  role = aws_iam_role.lambda_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["dynamodb:GetItem", "dynamodb:PutItem"]
      Resource = aws_dynamodb_table.my_table.arn
    }]
  })
}
```

### Limitations

- **LocalStack not supported**: The IAM Policy Simulator API
  (`SimulatePrincipalPolicy`) is not available in LocalStack. These tests
  require real AWS credentials.
- **Required IAM permissions**: The caller must have
  `iam:SimulatePrincipalPolicy` permission to use this data source.

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
