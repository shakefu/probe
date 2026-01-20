// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"github.com/aws/smithy-go"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	fwtypes "github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure IamPolicySimulationDataSource satisfies datasource interfaces.
var _ datasource.DataSource = &IamPolicySimulationDataSource{}
var _ datasource.DataSourceWithConfigure = &IamPolicySimulationDataSource{}

// IamPolicySimulationDataSource implements the probe_iam_policy_simulation data source.
type IamPolicySimulationDataSource struct {
	cfg aws.Config
}

// IamPolicySimulationDataSourceModel describes the data source data model.
type IamPolicySimulationDataSourceModel struct {
	// Required arguments
	PolicySourceArn fwtypes.String `tfsdk:"policy_source_arn"`
	Actions         fwtypes.List   `tfsdk:"actions"`

	// Optional arguments
	ResourceArns                fwtypes.List   `tfsdk:"resource_arns"`
	CallerArn                   fwtypes.String `tfsdk:"caller_arn"`
	Context                     fwtypes.Set    `tfsdk:"context"`
	AdditionalPolicies          fwtypes.List   `tfsdk:"additional_policies"`
	PermissionsBoundaryPolicies fwtypes.List   `tfsdk:"permissions_boundary_policies"`
	ResourcePolicy              fwtypes.String `tfsdk:"resource_policy"`

	// Computed attributes
	Allowed fwtypes.Bool   `tfsdk:"allowed"`
	Error   fwtypes.String `tfsdk:"error"`
	Results fwtypes.List   `tfsdk:"results"`
}

// ContextEntryModel represents a context entry for IAM policy simulation.
type ContextEntryModel struct {
	Key    fwtypes.String `tfsdk:"key"`
	Type   fwtypes.String `tfsdk:"type"`
	Values fwtypes.List   `tfsdk:"values"`
}

// SimulationResultModel represents a single simulation result.
type SimulationResultModel struct {
	Action             fwtypes.String `tfsdk:"action"`
	ResourceArn        fwtypes.String `tfsdk:"resource_arn"`
	Allowed            fwtypes.Bool   `tfsdk:"allowed"`
	Decision           fwtypes.String `tfsdk:"decision"`
	MatchedStatements  fwtypes.List   `tfsdk:"matched_statements"`
	MissingContextKeys fwtypes.List   `tfsdk:"missing_context_keys"`
}

// MatchedStatementModel represents a policy statement that matched.
type MatchedStatementModel struct {
	SourcePolicyId   fwtypes.String `tfsdk:"source_policy_id"`
	SourcePolicyType fwtypes.String `tfsdk:"source_policy_type"`
}

// NewIamPolicySimulationDataSource creates a new data source instance.
func NewIamPolicySimulationDataSource() datasource.DataSource {
	return &IamPolicySimulationDataSource{}
}

func (d *IamPolicySimulationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_policy_simulation"
}

// contextEntryAttrTypes returns the attribute types for a context entry.
func contextEntryAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"key":    fwtypes.StringType,
		"type":   fwtypes.StringType,
		"values": fwtypes.ListType{ElemType: fwtypes.StringType},
	}
}

// matchedStatementAttrTypes returns the attribute types for a matched statement.
func matchedStatementAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"source_policy_id":   fwtypes.StringType,
		"source_policy_type": fwtypes.StringType,
	}
}

// simulationResultAttrTypes returns the attribute types for a simulation result.
func simulationResultAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"action":               fwtypes.StringType,
		"resource_arn":         fwtypes.StringType,
		"allowed":              fwtypes.BoolType,
		"decision":             fwtypes.StringType,
		"matched_statements":   fwtypes.ListType{ElemType: fwtypes.ObjectType{AttrTypes: matchedStatementAttrTypes()}},
		"missing_context_keys": fwtypes.ListType{ElemType: fwtypes.StringType},
	}
}

func (d *IamPolicySimulationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Probes IAM permissions using the AWS Policy Simulator API without failing when permissions are denied.",

		Attributes: map[string]schema.Attribute{
			// Required arguments
			"policy_source_arn": schema.StringAttribute{
				Description: "ARN of the IAM principal (user, role, group) whose policies will be tested.",
				Required:    true,
			},
			"actions": schema.ListAttribute{
				Description: "List of IAM actions to simulate (e.g., s3:GetObject, iam:CreateUser).",
				Required:    true,
				ElementType: fwtypes.StringType,
			},

			// Optional arguments
			"resource_arns": schema.ListAttribute{
				Description: "List of resource ARNs to simulate against. Defaults to ['*'] if not specified.",
				Optional:    true,
				ElementType: fwtypes.StringType,
			},
			"caller_arn": schema.StringAttribute{
				Description: "ARN of user to simulate as caller. Defaults to policy_source_arn if it's a user.",
				Optional:    true,
			},
			"additional_policies": schema.ListAttribute{
				Description: "Additional IAM policies (as JSON strings) to include in simulation.",
				Optional:    true,
				ElementType: fwtypes.StringType,
			},
			"permissions_boundary_policies": schema.ListAttribute{
				Description: "Permissions boundary policies (as JSON strings) to apply.",
				Optional:    true,
				ElementType: fwtypes.StringType,
			},
			"resource_policy": schema.StringAttribute{
				Description: "Resource-based policy (as JSON string) for simulation.",
				Optional:    true,
			},

			// Computed attributes
			"allowed": schema.BoolAttribute{
				Description: "True if ALL actions are allowed for ALL resources. This is the primary output for simple permission checks.",
				Computed:    true,
			},
			"error": schema.StringAttribute{
				Description: "Error message if simulation could not be performed (e.g., principal doesn't exist). Null if successful.",
				Computed:    true,
			},
			"results": schema.ListNestedAttribute{
				Description: "Detailed results for each action/resource combination.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"action": schema.StringAttribute{
							Description: "The action that was evaluated.",
							Computed:    true,
						},
						"resource_arn": schema.StringAttribute{
							Description: "The resource ARN evaluated.",
							Computed:    true,
						},
						"allowed": schema.BoolAttribute{
							Description: "Whether this specific action/resource is allowed.",
							Computed:    true,
						},
						"decision": schema.StringAttribute{
							Description: "Decision: 'allowed', 'explicitDeny', or 'implicitDeny'.",
							Computed:    true,
						},
						"matched_statements": schema.ListNestedAttribute{
							Description: "Statements that contributed to the decision.",
							Computed:    true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"source_policy_id": schema.StringAttribute{
										Description: "The identifier of the policy that contains the statement.",
										Computed:    true,
									},
									"source_policy_type": schema.StringAttribute{
										Description: "The type of policy (e.g., user, group, role, aws-managed, user-managed).",
										Computed:    true,
									},
								},
							},
						},
						"missing_context_keys": schema.ListAttribute{
							Description: "Context keys required but not provided.",
							Computed:    true,
							ElementType: fwtypes.StringType,
						},
					},
				},
			},
		},

		// Context block
		Blocks: map[string]schema.Block{
			"context": schema.SetNestedBlock{
				Description: "Context entries for condition evaluation.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Description: "Context key name (e.g., 'aws:CurrentTime').",
							Required:    true,
						},
						"type": schema.StringAttribute{
							Description: "Value type: 'string', 'stringList', 'numeric', 'numericList', 'boolean', 'date', 'dateList', 'ip', 'ipList', 'binary', 'binaryList'.",
							Required:    true,
						},
						"values": schema.ListAttribute{
							Description: "Values for the context key.",
							Required:    true,
							ElementType: fwtypes.StringType,
						},
					},
				},
			},
		},
	}
}

func (d *IamPolicySimulationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	cfg, ok := req.ProviderData.(aws.Config)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected aws.Config, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.cfg = cfg
}

func (d *IamPolicySimulationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IamPolicySimulationDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create IAM client
	client := iam.NewFromConfig(d.cfg)

	// Build the simulation input
	input, diags := d.buildSimulationInput(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Call SimulatePrincipalPolicy with pagination
	var allResults []iamtypes.EvaluationResult
	paginator := iam.NewSimulatePrincipalPolicyPaginator(client, input)

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			// Handle specific error types per probe philosophy
			if isNoSuchEntityError(err) {
				data.Allowed = fwtypes.BoolValue(false)
				data.Error = fwtypes.StringValue(fmt.Sprintf("principal not found: %s", data.PolicySourceArn.ValueString()))
				data.Results = fwtypes.ListNull(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()})
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
			if isAccessDeniedError(err) {
				data.Allowed = fwtypes.BoolValue(false)
				data.Error = fwtypes.StringValue("access denied: caller lacks iam:SimulatePrincipalPolicy permission")
				data.Results = fwtypes.ListNull(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()})
				resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
				return
			}
			// Other errors are actual configuration problems
			resp.Diagnostics.AddError("IAM Policy Simulation Failed", err.Error())
			return
		}
		allResults = append(allResults, output.EvaluationResults...)
	}

	// Convert results to Terraform types
	results, diags := d.convertResults(ctx, allResults)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine overall allowed status (all actions must be allowed)
	allAllowed := true
	for _, result := range allResults {
		if result.EvalDecision != iamtypes.PolicyEvaluationDecisionTypeAllowed {
			allAllowed = false
			break
		}
	}

	data.Allowed = fwtypes.BoolValue(allAllowed)
	data.Error = fwtypes.StringNull()
	data.Results = results

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// buildSimulationInput constructs the SimulatePrincipalPolicyInput from the data model.
func (d *IamPolicySimulationDataSource) buildSimulationInput(ctx context.Context, data *IamPolicySimulationDataSourceModel) (*iam.SimulatePrincipalPolicyInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Extract actions list
	var actions []string
	diags.Append(data.Actions.ElementsAs(ctx, &actions, false)...)
	if diags.HasError() {
		return nil, diags
	}

	input := &iam.SimulatePrincipalPolicyInput{
		PolicySourceArn: aws.String(data.PolicySourceArn.ValueString()),
		ActionNames:     actions,
	}

	// Handle optional resource ARNs (default to "*" if not specified)
	if !data.ResourceArns.IsNull() && !data.ResourceArns.IsUnknown() {
		var resourceArns []string
		diags.Append(data.ResourceArns.ElementsAs(ctx, &resourceArns, false)...)
		if diags.HasError() {
			return nil, diags
		}
		input.ResourceArns = resourceArns
	} else {
		input.ResourceArns = []string{"*"}
	}

	// Handle optional caller ARN
	if !data.CallerArn.IsNull() && !data.CallerArn.IsUnknown() {
		input.CallerArn = aws.String(data.CallerArn.ValueString())
	}

	// Handle optional additional policies
	if !data.AdditionalPolicies.IsNull() && !data.AdditionalPolicies.IsUnknown() {
		var policies []string
		diags.Append(data.AdditionalPolicies.ElementsAs(ctx, &policies, false)...)
		if diags.HasError() {
			return nil, diags
		}
		input.PolicyInputList = policies
	}

	// Handle optional permissions boundary policies
	if !data.PermissionsBoundaryPolicies.IsNull() && !data.PermissionsBoundaryPolicies.IsUnknown() {
		var policies []string
		diags.Append(data.PermissionsBoundaryPolicies.ElementsAs(ctx, &policies, false)...)
		if diags.HasError() {
			return nil, diags
		}
		input.PermissionsBoundaryPolicyInputList = policies
	}

	// Handle optional resource policy
	if !data.ResourcePolicy.IsNull() && !data.ResourcePolicy.IsUnknown() {
		input.ResourcePolicy = aws.String(data.ResourcePolicy.ValueString())
	}

	// Handle context entries
	if !data.Context.IsNull() && !data.Context.IsUnknown() {
		var contextEntries []ContextEntryModel
		diags.Append(data.Context.ElementsAs(ctx, &contextEntries, false)...)
		if diags.HasError() {
			return nil, diags
		}

		var awsContextEntries []iamtypes.ContextEntry
		for _, entry := range contextEntries {
			var values []string
			diags.Append(entry.Values.ElementsAs(ctx, &values, false)...)
			if diags.HasError() {
				return nil, diags
			}

			contextKeyType, err := parseContextKeyType(entry.Type.ValueString())
			if err != nil {
				diags.AddError("Invalid Context Key Type", err.Error())
				return nil, diags
			}

			awsContextEntries = append(awsContextEntries, iamtypes.ContextEntry{
				ContextKeyName:   aws.String(entry.Key.ValueString()),
				ContextKeyType:   contextKeyType,
				ContextKeyValues: values,
			})
		}
		input.ContextEntries = awsContextEntries
	}

	return input, diags
}

// convertResults converts AWS EvaluationResults to Terraform list values.
func (d *IamPolicySimulationDataSource) convertResults(ctx context.Context, results []iamtypes.EvaluationResult) (fwtypes.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	if len(results) == 0 {
		return fwtypes.ListNull(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()}), diags
	}

	var resultValues []attr.Value
	for _, result := range results {
		// Convert matched statements
		var matchedStmtValues []attr.Value
		for _, stmt := range result.MatchedStatements {
			stmtObj, stmtDiags := fwtypes.ObjectValue(matchedStatementAttrTypes(), map[string]attr.Value{
				"source_policy_id":   fwtypes.StringValue(aws.ToString(stmt.SourcePolicyId)),
				"source_policy_type": fwtypes.StringValue(string(stmt.SourcePolicyType)),
			})
			diags.Append(stmtDiags...)
			if diags.HasError() {
				return fwtypes.ListNull(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()}), diags
			}
			matchedStmtValues = append(matchedStmtValues, stmtObj)
		}

		matchedStmtsList, stmtDiags := fwtypes.ListValue(fwtypes.ObjectType{AttrTypes: matchedStatementAttrTypes()}, matchedStmtValues)
		diags.Append(stmtDiags...)
		if diags.HasError() {
			return fwtypes.ListNull(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()}), diags
		}

		// Convert missing context keys
		var missingKeyValues []attr.Value
		for _, key := range result.MissingContextValues {
			missingKeyValues = append(missingKeyValues, fwtypes.StringValue(key))
		}
		missingKeysList, keyDiags := fwtypes.ListValue(fwtypes.StringType, missingKeyValues)
		diags.Append(keyDiags...)
		if diags.HasError() {
			return fwtypes.ListNull(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()}), diags
		}

		// Determine resource ARN (use "*" if not specified in result)
		resourceArn := "*"
		if result.EvalResourceName != nil {
			resourceArn = *result.EvalResourceName
		}

		// Build result object
		resultObj, objDiags := fwtypes.ObjectValue(simulationResultAttrTypes(), map[string]attr.Value{
			"action":               fwtypes.StringValue(aws.ToString(result.EvalActionName)),
			"resource_arn":         fwtypes.StringValue(resourceArn),
			"allowed":              fwtypes.BoolValue(result.EvalDecision == iamtypes.PolicyEvaluationDecisionTypeAllowed),
			"decision":             fwtypes.StringValue(decisionToString(result.EvalDecision)),
			"matched_statements":   matchedStmtsList,
			"missing_context_keys": missingKeysList,
		})
		diags.Append(objDiags...)
		if diags.HasError() {
			return fwtypes.ListNull(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()}), diags
		}

		resultValues = append(resultValues, resultObj)
	}

	resultsList, listDiags := fwtypes.ListValue(fwtypes.ObjectType{AttrTypes: simulationResultAttrTypes()}, resultValues)
	diags.Append(listDiags...)
	return resultsList, diags
}

// parseContextKeyType converts a string to an IAM ContextKeyTypeEnum.
func parseContextKeyType(typeStr string) (iamtypes.ContextKeyTypeEnum, error) {
	switch strings.ToLower(typeStr) {
	case "string":
		return iamtypes.ContextKeyTypeEnumString, nil
	case "stringlist":
		return iamtypes.ContextKeyTypeEnumStringList, nil
	case "numeric":
		return iamtypes.ContextKeyTypeEnumNumeric, nil
	case "numericlist":
		return iamtypes.ContextKeyTypeEnumNumericList, nil
	case "boolean":
		return iamtypes.ContextKeyTypeEnumBoolean, nil
	case "booleanlist":
		return iamtypes.ContextKeyTypeEnumBooleanList, nil
	case "date":
		return iamtypes.ContextKeyTypeEnumDate, nil
	case "datelist":
		return iamtypes.ContextKeyTypeEnumDateList, nil
	case "ip":
		return iamtypes.ContextKeyTypeEnumIp, nil
	case "iplist":
		return iamtypes.ContextKeyTypeEnumIpList, nil
	case "binary":
		return iamtypes.ContextKeyTypeEnumBinary, nil
	case "binarylist":
		return iamtypes.ContextKeyTypeEnumBinaryList, nil
	default:
		return "", fmt.Errorf("invalid context key type: %s", typeStr)
	}
}

// decisionToString converts an IAM PolicyEvaluationDecisionType to a string.
func decisionToString(decision iamtypes.PolicyEvaluationDecisionType) string {
	switch decision {
	case iamtypes.PolicyEvaluationDecisionTypeAllowed:
		return "allowed"
	case iamtypes.PolicyEvaluationDecisionTypeExplicitDeny:
		return "explicitDeny"
	case iamtypes.PolicyEvaluationDecisionTypeImplicitDeny:
		return "implicitDeny"
	default:
		return string(decision)
	}
}

// isNoSuchEntityError checks if the error is a NoSuchEntity error.
func isNoSuchEntityError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "NoSuchEntity"
	}
	return false
}

// isAccessDeniedError checks if the error is an AccessDenied error.
func isAccessDeniedError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return apiErr.ErrorCode() == "AccessDenied"
	}
	return false
}
