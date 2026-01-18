---
page_title: "probe Data Source - terraform-provider-probe"
subcategory: ""
description: |-
  Checks whether an AWS resource exists without failing when it doesn't.
---

# probe (Data Source)

Checks whether an AWS resource exists without failing when it doesn't.

This data source uses native AWS SDK calls to check for resource existence
and retrieve properties. Unlike standard AWS data sources that fail with an
error when a resource doesn't exist, this data source returns `exists = false`.

## Example Usage

### Basic existence check

```terraform
data "probe" "my_table" {
  type = "aws_dynamodb_table"
  id   = "my-table"
}

output "table_exists" {
  value = data.probe.my_table.exists
}
```

### Create-or-adopt pattern (Terraform)

```terraform
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

data "aws_dynamodb_table" "contacts" {
  count = data.probe.contacts_table.exists ? 1 : 0
  name  = "${var.prefix}-contacts"
}

output "contacts_table_arn" {
  value = (
    data.probe.contacts_table.exists
    ? data.aws_dynamodb_table.contacts[0].arn
    : aws_dynamodb_table.contacts[0].arn
  )
}
```

### Accessing Tags

```terraform
data "probe" "my_table" {
  type = "aws_dynamodb_table"
  id   = "my-table"
}

output "table_tags" {
  value = data.probe.my_table.exists ? data.probe.my_table.properties.Tags : null
}
```

## Schema

### Required

- `type` (String) Resource type. Accepts Terraform-style names
  (e.g., `aws_dynamodb_table`) or AWS-style type names
  (e.g., `AWS::DynamoDB::Table`).
- `id` (String) Resource identifier (table name, bucket name, etc.).

### Read-Only

- `exists` (Boolean) Whether the resource exists.
- `arn` (String) Resource ARN. Null if the resource does not exist.
- `properties` (Dynamic) Resource properties including Tags when available.
  Null if the resource does not exist.

## Supported Resource Types

The provider uses native AWS SDK calls for each resource type. Currently
supported:

| Terraform Type       | AWS Type               | Identifier     |
| -------------------- | ---------------------- | -------------- |
| `aws_dynamodb_table` | `AWS::DynamoDB::Table` | Table name     |
| `aws_s3_bucket`      | `AWS::S3::Bucket`      | Bucket name    |

Additional resource types will be added incrementally. Contributions welcome!
