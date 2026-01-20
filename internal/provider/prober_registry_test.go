// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestNormalizeTypeName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Direct mappings from normalizedTypes
		{
			name:     "terraform dynamodb",
			input:    "aws_dynamodb_table",
			expected: "aws_dynamodb_table",
		},
		{
			name:     "cloud control dynamodb",
			input:    "AWS::DynamoDB::Table",
			expected: "aws_dynamodb_table",
		},
		{
			name:     "short form dynamodb",
			input:    "dynamodb_table",
			expected: "aws_dynamodb_table",
		},
		{
			name:     "global table maps to dynamodb",
			input:    "AWS::DynamoDB::GlobalTable",
			expected: "aws_dynamodb_table",
		},
		{
			name:     "terraform s3",
			input:    "aws_s3_bucket",
			expected: "aws_s3_bucket",
		},
		{
			name:     "cloud control s3",
			input:    "AWS::S3::Bucket",
			expected: "aws_s3_bucket",
		},
		{
			name:     "short form s3",
			input:    "s3_bucket",
			expected: "aws_s3_bucket",
		},

		// Unknown types with aws_ prefix pass through
		{
			name:     "unknown aws type passes through",
			input:    "aws_unknown_resource",
			expected: "aws_unknown_resource",
		},

		// Cloud Control format conversion for unknown types
		{
			name:     "unknown cloud control type converts",
			input:    "AWS::Lambda::Function",
			expected: "aws_lambda_function",
		},
		{
			name:     "unknown cloud control with mixed case",
			input:    "AWS::IAM::Role",
			expected: "aws_iam_role",
		},

		// Malformed Cloud Control (not 3 parts) returns as-is
		{
			name:     "malformed cloud control too few parts",
			input:    "AWS::Service",
			expected: "AWS::Service",
		},
		{
			name:     "malformed cloud control too many parts",
			input:    "AWS::Service::Resource::Extra",
			expected: "AWS::Service::Resource::Extra",
		},

		// Unknown format returns as-is
		{
			name:     "completely unknown format",
			input:    "some_random_type",
			expected: "some_random_type",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeTypeName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeTypeName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestProberRegistry_GetProber(t *testing.T) {
	cfg := aws.Config{Region: "us-east-1"}
	registry := NewProberRegistry(cfg)

	t.Run("returns prober for supported type", func(t *testing.T) {
		prober, err := registry.GetProber("aws_dynamodb_table")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prober == nil {
			t.Fatal("expected prober to be non-nil")
		}
	})

	t.Run("returns same prober instance on repeated calls", func(t *testing.T) {
		prober1, _ := registry.GetProber("aws_dynamodb_table")
		prober2, _ := registry.GetProber("aws_dynamodb_table")
		if prober1 != prober2 {
			t.Error("expected same prober instance to be returned")
		}
	})

	t.Run("normalizes type names", func(t *testing.T) {
		// Get prober using Cloud Control syntax
		prober1, err := registry.GetProber("AWS::DynamoDB::Table")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should return same prober as terraform syntax
		prober2, _ := registry.GetProber("aws_dynamodb_table")
		if prober1 != prober2 {
			t.Error("expected same prober for equivalent type names")
		}
	})

	t.Run("returns error for unsupported type", func(t *testing.T) {
		_, err := registry.GetProber("aws_unsupported_resource")
		if err == nil {
			t.Fatal("expected error for unsupported type")
		}
	})

	t.Run("supports S3 bucket type", func(t *testing.T) {
		prober, err := registry.GetProber("aws_s3_bucket")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if prober == nil {
			t.Fatal("expected prober to be non-nil")
		}
	})
}

func TestProberRegistry_SupportedTypes(t *testing.T) {
	cfg := aws.Config{Region: "us-east-1"}
	registry := NewProberRegistry(cfg)

	types := registry.SupportedTypes()

	if len(types) == 0 {
		t.Fatal("expected at least one supported type")
	}

	// Check that expected types are present
	typeSet := make(map[string]bool)
	for _, typ := range types {
		typeSet[typ] = true
	}

	expectedTypes := []string{"aws_dynamodb_table", "aws_s3_bucket"}
	for _, expected := range expectedTypes {
		if !typeSet[expected] {
			t.Errorf("expected %q to be in supported types", expected)
		}
	}

	// Check that there are no duplicates
	if len(types) != len(typeSet) {
		t.Error("SupportedTypes() returned duplicate entries")
	}
}

func TestNewProberRegistry(t *testing.T) {
	cfg := aws.Config{Region: "us-west-2"}
	registry := NewProberRegistry(cfg)

	if registry == nil {
		t.Fatal("expected registry to be non-nil")
	}

	if registry.probers == nil {
		t.Error("expected probers map to be initialized")
	}
}
