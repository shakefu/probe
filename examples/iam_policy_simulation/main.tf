# IAM Policy Simulation Example
#
# This example demonstrates using the probe_iam_policy_simulation data source
# to check permissions before creating resources or taking conditional actions.
#
# The probe provider philosophy: check without failing. Instead of errors,
# you get results that can be used in conditional logic.

terraform {
  required_providers {
    probe = {
      source  = "shakefu/probe"
      version = "~> 0.1"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "probe" {}
provider "aws" {}

# Example 1: Simple Permission Check
# ----------------------------------
# Check if an existing role has the permissions we need

variable "role_arn" {
  description = "ARN of the IAM role to check permissions for"
  type        = string
  default     = "" # Set this to test with a real role
}

data "probe_iam_policy_simulation" "s3_permissions" {
  count = var.role_arn != "" ? 1 : 0

  policy_source_arn = var.role_arn
  actions           = ["s3:GetObject", "s3:PutObject", "s3:ListBucket"]
  resource_arns     = ["arn:aws:s3:::example-bucket/*"]
}

output "s3_access_allowed" {
  description = "Whether the role has S3 access permissions"
  value       = var.role_arn != "" ? data.probe_iam_policy_simulation.s3_permissions[0].allowed : null
}

# Example 2: Create-or-Skip Pattern
# ---------------------------------
# Only create a policy attachment if the role doesn't already have permissions

resource "aws_iam_role" "example" {
  name = "probe-iam-simulation-example-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "lambda.amazonaws.com"
      }
    }]
  })
}

# Check what permissions the role currently has
data "probe_iam_policy_simulation" "current_permissions" {
  policy_source_arn = aws_iam_role.example.arn
  actions           = ["logs:CreateLogGroup", "logs:CreateLogStream", "logs:PutLogEvents"]
  resource_arns     = ["*"]

  depends_on = [aws_iam_role.example]
}

# Only attach logging policy if the role doesn't already have logging permissions
resource "aws_iam_role_policy" "logging" {
  count = data.probe_iam_policy_simulation.current_permissions.allowed ? 0 : 1

  name = "logging-permissions"
  role = aws_iam_role.example.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ]
      Resource = "*"
    }]
  })
}

output "logging_policy_created" {
  description = "Whether a new logging policy was created (true if role lacked permissions)"
  value       = length(aws_iam_role_policy.logging) > 0
}

# Example 3: Detailed Results Analysis
# ------------------------------------
# Get detailed information about each permission check

data "probe_iam_policy_simulation" "detailed_check" {
  policy_source_arn = aws_iam_role.example.arn
  actions = [
    "dynamodb:GetItem",
    "dynamodb:PutItem",
    "dynamodb:DeleteItem",
    "dynamodb:Query"
  ]
  resource_arns = ["arn:aws:dynamodb:*:*:table/example-table"]

  depends_on = [aws_iam_role.example]
}

output "dynamodb_permissions_summary" {
  description = "Summary of DynamoDB permission check results"
  value = {
    all_allowed = data.probe_iam_policy_simulation.detailed_check.allowed
    error       = data.probe_iam_policy_simulation.detailed_check.error
    results = [
      for r in data.probe_iam_policy_simulation.detailed_check.results : {
        action   = r.action
        allowed  = r.allowed
        decision = r.decision
      }
    ]
  }
}

# Example 4: IP-Based Context Conditions
# --------------------------------------
# Test permissions with context conditions (e.g., IP address restrictions)

data "probe_iam_policy_simulation" "with_ip_context" {
  policy_source_arn = aws_iam_role.example.arn
  actions           = ["s3:GetObject"]
  resource_arns     = ["arn:aws:s3:::example-bucket/*"]

  context {
    key    = "aws:SourceIp"
    type   = "ip"
    values = ["192.0.2.0/24"] # Example IP range
  }

  depends_on = [aws_iam_role.example]
}

output "ip_restricted_access" {
  description = "Whether access is allowed from the specified IP range"
  value       = data.probe_iam_policy_simulation.with_ip_context.allowed
}
