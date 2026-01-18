terraform {
  required_providers {
    probe = {
      source = "registry.terraform.io/shakefu/probe"
    }
  }
}

# Provider auto-detects LocalStack at localhost:4566
# Or explicitly enable it:
provider "probe" {
  # localstack = true
  # endpoint = "http://localhost:4566"
}

# Check for supported resource types
data "probe" "dynamodb_table" {
  type = "aws_dynamodb_table"
  id   = "test-table"
}

data "probe" "s3_bucket" {
  type = "aws_s3_bucket"
  id   = "test-bucket"
}

# You can also use AWS CloudFormation type names
data "probe" "dynamodb_cfn_style" {
  type = "AWS::DynamoDB::Table"
  id   = "another-table"
}

output "resources" {
  value = {
    dynamodb_exists     = data.probe.dynamodb_table.exists
    s3_exists           = data.probe.s3_bucket.exists
    dynamodb_cfn_exists = data.probe.dynamodb_cfn_style.exists
  }
}
