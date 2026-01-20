// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwtypes "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestIamPolicySimulationDataSource_Schema(t *testing.T) {
	ds := NewIamPolicySimulationDataSource()

	req := datasource.SchemaRequest{}
	resp := &datasource.SchemaResponse{}

	ds.Schema(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Schema returned errors: %v", resp.Diagnostics)
	}

	schema := resp.Schema

	// Verify required attributes
	if _, ok := schema.Attributes["policy_source_arn"]; !ok {
		t.Error("expected policy_source_arn attribute")
	}
	if _, ok := schema.Attributes["actions"]; !ok {
		t.Error("expected actions attribute")
	}

	// Verify computed attributes
	if _, ok := schema.Attributes["allowed"]; !ok {
		t.Error("expected allowed attribute")
	}
	if _, ok := schema.Attributes["error"]; !ok {
		t.Error("expected error attribute")
	}
	if _, ok := schema.Attributes["results"]; !ok {
		t.Error("expected results attribute")
	}

	// Verify optional attributes
	if _, ok := schema.Attributes["resource_arns"]; !ok {
		t.Error("expected resource_arns attribute")
	}
	if _, ok := schema.Attributes["caller_arn"]; !ok {
		t.Error("expected caller_arn attribute")
	}

	// Verify context block
	if _, ok := schema.Blocks["context"]; !ok {
		t.Error("expected context block")
	}
}

func TestIamPolicySimulationDataSource_Metadata(t *testing.T) {
	ds := NewIamPolicySimulationDataSource()

	req := datasource.MetadataRequest{
		ProviderTypeName: "probe",
	}
	resp := &datasource.MetadataResponse{}

	ds.Metadata(context.Background(), req, resp)

	expected := "probe_iam_policy_simulation"
	if resp.TypeName != expected {
		t.Errorf("expected TypeName=%q, got %q", expected, resp.TypeName)
	}
}

func TestParseContextKeyType(t *testing.T) {
	tests := []struct {
		input    string
		expected iamtypes.ContextKeyTypeEnum
		wantErr  bool
	}{
		{"string", iamtypes.ContextKeyTypeEnumString, false},
		{"stringList", iamtypes.ContextKeyTypeEnumStringList, false},
		{"STRING", iamtypes.ContextKeyTypeEnumString, false},
		{"STRINGLIST", iamtypes.ContextKeyTypeEnumStringList, false},
		{"numeric", iamtypes.ContextKeyTypeEnumNumeric, false},
		{"numericList", iamtypes.ContextKeyTypeEnumNumericList, false},
		{"boolean", iamtypes.ContextKeyTypeEnumBoolean, false},
		{"booleanList", iamtypes.ContextKeyTypeEnumBooleanList, false},
		{"date", iamtypes.ContextKeyTypeEnumDate, false},
		{"dateList", iamtypes.ContextKeyTypeEnumDateList, false},
		{"ip", iamtypes.ContextKeyTypeEnumIp, false},
		{"ipList", iamtypes.ContextKeyTypeEnumIpList, false},
		{"binary", iamtypes.ContextKeyTypeEnumBinary, false},
		{"binaryList", iamtypes.ContextKeyTypeEnumBinaryList, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseContextKeyType(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDecisionToString(t *testing.T) {
	tests := []struct {
		input    iamtypes.PolicyEvaluationDecisionType
		expected string
	}{
		{iamtypes.PolicyEvaluationDecisionTypeAllowed, "allowed"},
		{iamtypes.PolicyEvaluationDecisionTypeExplicitDeny, "explicitDeny"},
		{iamtypes.PolicyEvaluationDecisionTypeImplicitDeny, "implicitDeny"},
	}

	for _, tt := range tests {
		t.Run(string(tt.input), func(t *testing.T) {
			result := decisionToString(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSimulationResultAttrTypes(t *testing.T) {
	types := simulationResultAttrTypes()

	// Verify all expected keys exist
	expectedKeys := []string{
		"action",
		"resource_arn",
		"allowed",
		"decision",
		"matched_statements",
		"missing_context_keys",
	}

	for _, key := range expectedKeys {
		if _, ok := types[key]; !ok {
			t.Errorf("expected key %q in simulationResultAttrTypes", key)
		}
	}
}

func TestMatchedStatementAttrTypes(t *testing.T) {
	types := matchedStatementAttrTypes()

	expectedKeys := []string{
		"source_policy_id",
		"source_policy_type",
	}

	for _, key := range expectedKeys {
		if _, ok := types[key]; !ok {
			t.Errorf("expected key %q in matchedStatementAttrTypes", key)
		}
	}
}

func TestConvertResults_EmptyResults(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	results, diags := ds.convertResults(context.Background(), []iamtypes.EvaluationResult{})

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if !results.IsNull() {
		t.Error("expected null list for empty results")
	}
}

func TestConvertResults_SingleAllowed(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	awsResults := []iamtypes.EvaluationResult{
		{
			EvalActionName:   aws.String("s3:GetObject"),
			EvalResourceName: aws.String("arn:aws:s3:::my-bucket/*"),
			EvalDecision:     iamtypes.PolicyEvaluationDecisionTypeAllowed,
			MatchedStatements: []iamtypes.Statement{
				{
					SourcePolicyId:   aws.String("policy-123"),
					SourcePolicyType: iamtypes.PolicySourceTypeUser,
				},
			},
			MissingContextValues: []string{},
		},
	}

	results, diags := ds.convertResults(context.Background(), awsResults)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if results.IsNull() {
		t.Fatal("expected non-null list")
	}

	elements := results.Elements()
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}

	// Check the result object
	resultObj, ok := elements[0].(fwtypes.Object)
	if !ok {
		t.Fatalf("expected Object type, got %T", elements[0])
	}

	attrs := resultObj.Attributes()

	action, ok := attrs["action"].(fwtypes.String)
	if !ok {
		t.Fatalf("expected action to be String, got %T", attrs["action"])
	}
	if action.ValueString() != "s3:GetObject" {
		t.Errorf("expected action=%q, got %q", "s3:GetObject", action.ValueString())
	}

	allowed, ok := attrs["allowed"].(fwtypes.Bool)
	if !ok {
		t.Fatalf("expected allowed to be Bool, got %T", attrs["allowed"])
	}
	if !allowed.ValueBool() {
		t.Error("expected allowed=true")
	}

	decision, ok := attrs["decision"].(fwtypes.String)
	if !ok {
		t.Fatalf("expected decision to be String, got %T", attrs["decision"])
	}
	if decision.ValueString() != "allowed" {
		t.Errorf("expected decision=%q, got %q", "allowed", decision.ValueString())
	}
}

func TestConvertResults_SingleDenied(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	awsResults := []iamtypes.EvaluationResult{
		{
			EvalActionName:       aws.String("iam:CreateUser"),
			EvalResourceName:     aws.String("*"),
			EvalDecision:         iamtypes.PolicyEvaluationDecisionTypeImplicitDeny,
			MatchedStatements:    []iamtypes.Statement{},
			MissingContextValues: []string{"aws:PrincipalAccount"},
		},
	}

	results, diags := ds.convertResults(context.Background(), awsResults)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	elements := results.Elements()
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}

	resultObj := elements[0].(fwtypes.Object)
	attrs := resultObj.Attributes()

	allowed := attrs["allowed"].(fwtypes.Bool)
	if allowed.ValueBool() {
		t.Error("expected allowed=false for denied action")
	}

	decision := attrs["decision"].(fwtypes.String)
	if decision.ValueString() != "implicitDeny" {
		t.Errorf("expected decision=%q, got %q", "implicitDeny", decision.ValueString())
	}

	missingKeys := attrs["missing_context_keys"].(fwtypes.List)
	keyElements := missingKeys.Elements()
	if len(keyElements) != 1 {
		t.Fatalf("expected 1 missing context key, got %d", len(keyElements))
	}
}

func TestConvertResults_ExplicitDeny(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	awsResults := []iamtypes.EvaluationResult{
		{
			EvalActionName:   aws.String("s3:DeleteBucket"),
			EvalResourceName: aws.String("arn:aws:s3:::protected-bucket"),
			EvalDecision:     iamtypes.PolicyEvaluationDecisionTypeExplicitDeny,
			MatchedStatements: []iamtypes.Statement{
				{
					SourcePolicyId:   aws.String("deny-policy"),
					SourcePolicyType: iamtypes.PolicySourceTypeAwsManaged,
				},
			},
		},
	}

	results, diags := ds.convertResults(context.Background(), awsResults)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	elements := results.Elements()
	resultObj := elements[0].(fwtypes.Object)
	attrs := resultObj.Attributes()

	decision := attrs["decision"].(fwtypes.String)
	if decision.ValueString() != "explicitDeny" {
		t.Errorf("expected decision=%q, got %q", "explicitDeny", decision.ValueString())
	}

	matchedStmts := attrs["matched_statements"].(fwtypes.List)
	stmtElements := matchedStmts.Elements()
	if len(stmtElements) != 1 {
		t.Fatalf("expected 1 matched statement, got %d", len(stmtElements))
	}

	stmtObj := stmtElements[0].(fwtypes.Object)
	stmtAttrs := stmtObj.Attributes()

	policyId := stmtAttrs["source_policy_id"].(fwtypes.String)
	if policyId.ValueString() != "deny-policy" {
		t.Errorf("expected source_policy_id=%q, got %q", "deny-policy", policyId.ValueString())
	}
}

func TestBuildSimulationInput_RequiredOnly(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	actionsList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{"s3:GetObject", "s3:PutObject"})

	data := &IamPolicySimulationDataSourceModel{
		PolicySourceArn:             fwtypes.StringValue("arn:aws:iam::123456789012:role/test-role"),
		Actions:                     actionsList,
		ResourceArns:                fwtypes.ListNull(fwtypes.StringType),
		CallerArn:                   fwtypes.StringNull(),
		Context:                     fwtypes.SetNull(fwtypes.ObjectType{AttrTypes: contextEntryAttrTypes()}),
		AdditionalPolicies:          fwtypes.ListNull(fwtypes.StringType),
		PermissionsBoundaryPolicies: fwtypes.ListNull(fwtypes.StringType),
		ResourcePolicy:              fwtypes.StringNull(),
	}

	input, diags := ds.buildSimulationInput(context.Background(), data)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if *input.PolicySourceArn != "arn:aws:iam::123456789012:role/test-role" {
		t.Errorf("expected PolicySourceArn=%q, got %q", "arn:aws:iam::123456789012:role/test-role", *input.PolicySourceArn)
	}

	if len(input.ActionNames) != 2 {
		t.Errorf("expected 2 actions, got %d", len(input.ActionNames))
	}

	// When ResourceArns is null, it should default to "*"
	if len(input.ResourceArns) != 1 || input.ResourceArns[0] != "*" {
		t.Errorf("expected ResourceArns=[*], got %v", input.ResourceArns)
	}

	if input.CallerArn != nil {
		t.Error("expected CallerArn to be nil")
	}
}

func TestBuildSimulationInput_WithResourceArns(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	actionsList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{"s3:GetObject"})
	resourceArnsList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{
		"arn:aws:s3:::bucket1/*",
		"arn:aws:s3:::bucket2/*",
	})

	data := &IamPolicySimulationDataSourceModel{
		PolicySourceArn:             fwtypes.StringValue("arn:aws:iam::123456789012:role/test-role"),
		Actions:                     actionsList,
		ResourceArns:                resourceArnsList,
		CallerArn:                   fwtypes.StringNull(),
		Context:                     fwtypes.SetNull(fwtypes.ObjectType{AttrTypes: contextEntryAttrTypes()}),
		AdditionalPolicies:          fwtypes.ListNull(fwtypes.StringType),
		PermissionsBoundaryPolicies: fwtypes.ListNull(fwtypes.StringType),
		ResourcePolicy:              fwtypes.StringNull(),
	}

	input, diags := ds.buildSimulationInput(context.Background(), data)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if len(input.ResourceArns) != 2 {
		t.Errorf("expected 2 resource ARNs, got %d", len(input.ResourceArns))
	}
}

func TestBuildSimulationInput_WithCallerArn(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	actionsList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{"s3:GetObject"})

	data := &IamPolicySimulationDataSourceModel{
		PolicySourceArn:             fwtypes.StringValue("arn:aws:iam::123456789012:role/test-role"),
		Actions:                     actionsList,
		ResourceArns:                fwtypes.ListNull(fwtypes.StringType),
		CallerArn:                   fwtypes.StringValue("arn:aws:iam::123456789012:user/test-user"),
		Context:                     fwtypes.SetNull(fwtypes.ObjectType{AttrTypes: contextEntryAttrTypes()}),
		AdditionalPolicies:          fwtypes.ListNull(fwtypes.StringType),
		PermissionsBoundaryPolicies: fwtypes.ListNull(fwtypes.StringType),
		ResourcePolicy:              fwtypes.StringNull(),
	}

	input, diags := ds.buildSimulationInput(context.Background(), data)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if input.CallerArn == nil {
		t.Fatal("expected CallerArn to be set")
	}

	if *input.CallerArn != "arn:aws:iam::123456789012:user/test-user" {
		t.Errorf("expected CallerArn=%q, got %q", "arn:aws:iam::123456789012:user/test-user", *input.CallerArn)
	}
}

func TestBuildSimulationInput_WithContext(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	actionsList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{"s3:GetObject"})

	// Build context entry
	valuesList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{"10.0.0.1"})

	contextEntry, _ := fwtypes.ObjectValue(contextEntryAttrTypes(), map[string]attr.Value{
		"key":    fwtypes.StringValue("aws:SourceIp"),
		"type":   fwtypes.StringValue("ip"),
		"values": valuesList,
	})

	contextSet, _ := fwtypes.SetValue(fwtypes.ObjectType{AttrTypes: contextEntryAttrTypes()}, []attr.Value{contextEntry})

	data := &IamPolicySimulationDataSourceModel{
		PolicySourceArn:             fwtypes.StringValue("arn:aws:iam::123456789012:role/test-role"),
		Actions:                     actionsList,
		ResourceArns:                fwtypes.ListNull(fwtypes.StringType),
		CallerArn:                   fwtypes.StringNull(),
		Context:                     contextSet,
		AdditionalPolicies:          fwtypes.ListNull(fwtypes.StringType),
		PermissionsBoundaryPolicies: fwtypes.ListNull(fwtypes.StringType),
		ResourcePolicy:              fwtypes.StringNull(),
	}

	input, diags := ds.buildSimulationInput(context.Background(), data)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if len(input.ContextEntries) != 1 {
		t.Fatalf("expected 1 context entry, got %d", len(input.ContextEntries))
	}

	entry := input.ContextEntries[0]
	if *entry.ContextKeyName != "aws:SourceIp" {
		t.Errorf("expected ContextKeyName=%q, got %q", "aws:SourceIp", *entry.ContextKeyName)
	}

	if entry.ContextKeyType != iamtypes.ContextKeyTypeEnumIp {
		t.Errorf("expected ContextKeyType=%v, got %v", iamtypes.ContextKeyTypeEnumIp, entry.ContextKeyType)
	}

	if len(entry.ContextKeyValues) != 1 || entry.ContextKeyValues[0] != "10.0.0.1" {
		t.Errorf("expected ContextKeyValues=[10.0.0.1], got %v", entry.ContextKeyValues)
	}
}

func TestBuildSimulationInput_WithAdditionalPolicies(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	actionsList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{"s3:GetObject"})
	policyJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:*","Resource":"*"}]}`
	policiesList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{policyJSON})

	data := &IamPolicySimulationDataSourceModel{
		PolicySourceArn:             fwtypes.StringValue("arn:aws:iam::123456789012:role/test-role"),
		Actions:                     actionsList,
		ResourceArns:                fwtypes.ListNull(fwtypes.StringType),
		CallerArn:                   fwtypes.StringNull(),
		Context:                     fwtypes.SetNull(fwtypes.ObjectType{AttrTypes: contextEntryAttrTypes()}),
		AdditionalPolicies:          policiesList,
		PermissionsBoundaryPolicies: fwtypes.ListNull(fwtypes.StringType),
		ResourcePolicy:              fwtypes.StringNull(),
	}

	input, diags := ds.buildSimulationInput(context.Background(), data)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if len(input.PolicyInputList) != 1 {
		t.Errorf("expected 1 additional policy, got %d", len(input.PolicyInputList))
	}

	if input.PolicyInputList[0] != policyJSON {
		t.Errorf("expected policy JSON to match")
	}
}

func TestBuildSimulationInput_WithResourcePolicy(t *testing.T) {
	ds := &IamPolicySimulationDataSource{}

	actionsList, _ := fwtypes.ListValueFrom(context.Background(), fwtypes.StringType, []string{"s3:GetObject"})
	resourcePolicyJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":"*","Action":"s3:GetObject","Resource":"arn:aws:s3:::bucket/*"}]}`

	data := &IamPolicySimulationDataSourceModel{
		PolicySourceArn:             fwtypes.StringValue("arn:aws:iam::123456789012:role/test-role"),
		Actions:                     actionsList,
		ResourceArns:                fwtypes.ListNull(fwtypes.StringType),
		CallerArn:                   fwtypes.StringNull(),
		Context:                     fwtypes.SetNull(fwtypes.ObjectType{AttrTypes: contextEntryAttrTypes()}),
		AdditionalPolicies:          fwtypes.ListNull(fwtypes.StringType),
		PermissionsBoundaryPolicies: fwtypes.ListNull(fwtypes.StringType),
		ResourcePolicy:              fwtypes.StringValue(resourcePolicyJSON),
	}

	input, diags := ds.buildSimulationInput(context.Background(), data)

	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if input.ResourcePolicy == nil {
		t.Fatal("expected ResourcePolicy to be set")
	}

	if *input.ResourcePolicy != resourcePolicyJSON {
		t.Error("expected ResourcePolicy to match input JSON")
	}
}

// testAccIamPolicySimulationPreCheck validates that acceptance tests can run.
// LocalStack does not support SimulatePrincipalPolicy, so these tests require
// real AWS credentials with iam:SimulatePrincipalPolicy permission.
func testAccIamPolicySimulationPreCheck(t *testing.T) {
	t.Helper()

	// LocalStack does not support SimulatePrincipalPolicy
	if localStackRunning() {
		t.Skip("Skipping IAM policy simulation test: LocalStack does not support SimulatePrincipalPolicy")
	}

	// Check for AWS credentials
	if os.Getenv("AWS_PROFILE") == "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		t.Skip("Skipping IAM policy simulation test: AWS credentials not configured")
	}
}

// TestAccIamPolicySimulation_AllowedAction tests that a role with S3 permissions
// shows allowed=true when simulating allowed actions.
func TestAccIamPolicySimulation_AllowedAction(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccIamPolicySimulationPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: "~> 5.0",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccIamPolicySimulationConfig_allowed,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.probe_iam_policy_simulation.test", "allowed", "true"),
					resource.TestCheckNoResourceAttr("data.probe_iam_policy_simulation.test", "error"),
					resource.TestCheckResourceAttrSet("data.probe_iam_policy_simulation.test", "results.#"),
				),
			},
		},
	})
}

const testAccIamPolicySimulationConfig_allowed = `
provider "aws" {}

resource "aws_iam_role" "test" {
  name = "probe-iam-simulation-test-role"

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

resource "aws_iam_role_policy" "test" {
  name = "probe-iam-simulation-test-policy"
  role = aws_iam_role.test.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:GetObject", "s3:ListBucket"]
      Resource = "*"
    }]
  })
}

data "probe_iam_policy_simulation" "test" {
  policy_source_arn = aws_iam_role.test.arn
  actions           = ["s3:GetObject"]
  resource_arns     = ["arn:aws:s3:::test-bucket/*"]

  depends_on = [aws_iam_role_policy.test]
}
`

// TestAccIamPolicySimulation_DeniedAction tests that missing permissions
// show allowed=false with implicitDeny decision.
func TestAccIamPolicySimulation_DeniedAction(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccIamPolicySimulationPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: "~> 5.0",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccIamPolicySimulationConfig_denied,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.probe_iam_policy_simulation.test", "allowed", "false"),
					resource.TestCheckNoResourceAttr("data.probe_iam_policy_simulation.test", "error"),
					resource.TestCheckResourceAttrSet("data.probe_iam_policy_simulation.test", "results.#"),
					resource.TestCheckResourceAttr("data.probe_iam_policy_simulation.test", "results.0.decision", "implicitDeny"),
				),
			},
		},
	})
}

const testAccIamPolicySimulationConfig_denied = `
provider "aws" {}

resource "aws_iam_role" "test" {
  name = "probe-iam-simulation-test-denied-role"

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

# Role has no policies attached, so all actions should be denied

data "probe_iam_policy_simulation" "test" {
  policy_source_arn = aws_iam_role.test.arn
  actions           = ["iam:CreateUser"]

  depends_on = [aws_iam_role.test]
}
`

// TestAccIamPolicySimulation_NonexistentPrincipal tests that a nonexistent
// principal returns allowed=false with an error message.
func TestAccIamPolicySimulation_NonexistentPrincipal(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccIamPolicySimulationPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccIamPolicySimulationConfig_nonexistent,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.probe_iam_policy_simulation.test", "allowed", "false"),
					resource.TestCheckResourceAttrSet("data.probe_iam_policy_simulation.test", "error"),
				),
			},
		},
	})
}

const testAccIamPolicySimulationConfig_nonexistent = `
data "probe_iam_policy_simulation" "test" {
  policy_source_arn = "arn:aws:iam::123456789012:role/nonexistent-role-that-does-not-exist"
  actions           = ["s3:GetObject"]
}
`

// TestAccIamPolicySimulation_MultipleActions tests simulation with multiple actions.
func TestAccIamPolicySimulation_MultipleActions(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccIamPolicySimulationPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		ExternalProviders: map[string]resource.ExternalProvider{
			"aws": {
				Source:            "hashicorp/aws",
				VersionConstraint: "~> 5.0",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccIamPolicySimulationConfig_multipleActions,
				Check: resource.ComposeAggregateTestCheckFunc(
					// Overall should be false because iam:CreateUser is denied
					resource.TestCheckResourceAttr("data.probe_iam_policy_simulation.test", "allowed", "false"),
					// Should have 2 results (one for each action)
					resource.TestCheckResourceAttr("data.probe_iam_policy_simulation.test", "results.#", "2"),
				),
			},
		},
	})
}

const testAccIamPolicySimulationConfig_multipleActions = `
provider "aws" {}

resource "aws_iam_role" "test" {
  name = "probe-iam-simulation-test-multi-role"

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

resource "aws_iam_role_policy" "test" {
  name = "probe-iam-simulation-test-multi-policy"
  role = aws_iam_role.test.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["s3:GetObject"]
      Resource = "*"
    }]
  })
}

data "probe_iam_policy_simulation" "test" {
  policy_source_arn = aws_iam_role.test.arn
  actions           = ["s3:GetObject", "iam:CreateUser"]

  depends_on = [aws_iam_role_policy.test]
}
`
