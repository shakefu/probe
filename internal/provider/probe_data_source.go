// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudcontrol"
	cctypes "github.com/aws/aws-sdk-go-v2/service/cloudcontrol/types"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ProbeDataSource satisfies various datasource interfaces.
var _ datasource.DataSource = &ProbeDataSource{}
var _ datasource.DataSourceWithConfigure = &ProbeDataSource{}

// ProbeDataSource defines the data source implementation.
type ProbeDataSource struct {
	client *cloudcontrol.Client
}

// ProbeDataSourceModel describes the data source data model.
type ProbeDataSourceModel struct {
	Type       types.String  `tfsdk:"type"`
	ID         types.String  `tfsdk:"id"`
	Exists     types.Bool    `tfsdk:"exists"`
	Arn        types.String  `tfsdk:"arn"`
	Properties types.Dynamic `tfsdk:"properties"`
}

func NewProbeDataSource() datasource.DataSource {
	return &ProbeDataSource{}
}

func (d *ProbeDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName
}

func (d *ProbeDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Checks whether an AWS resource exists without failing when it doesn't.",

		Attributes: map[string]schema.Attribute{
			"type": schema.StringAttribute{
				Description: "Resource type (e.g., aws_dynamodb_table or AWS::DynamoDB::Table).",
				Required:    true,
			},
			"id": schema.StringAttribute{
				Description: "Resource identifier (table name, bucket name, etc.).",
				Required:    true,
			},
			"exists": schema.BoolAttribute{
				Description: "Whether the resource exists.",
				Computed:    true,
			},
			"arn": schema.StringAttribute{
				Description: "Resource ARN (null if resource does not exist).",
				Computed:    true,
			},
			"properties": schema.DynamicAttribute{
				Description: "Resource properties as a map (null if resource does not exist).",
				Computed:    true,
			},
		},
	}
}

func (d *ProbeDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*cloudcontrol.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *cloudcontrol.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ProbeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProbeDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudControlType := resolveCloudControlType(data.Type.ValueString())
	identifier := data.ID.ValueString()

	// Use ListResources and filter by identifier for LocalStack compatibility.
	// GetResource is not implemented in LocalStack.
	var foundResource *cctypes.ResourceDescription
	paginator := cloudcontrol.NewListResourcesPaginator(d.client, &cloudcontrol.ListResourcesInput{
		TypeName: aws.String(cloudControlType),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Probe failed", err.Error())
			return
		}

		for _, resource := range page.ResourceDescriptions {
			if resource.Identifier != nil && *resource.Identifier == identifier {
				foundResource = &resource
				break
			}
		}

		if foundResource != nil {
			break
		}
	}

	if foundResource == nil {
		// Resource not found - NOT AN ERROR
		data.Exists = types.BoolValue(false)
		data.Arn = types.StringNull()
		data.Properties = types.DynamicNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Parse properties JSON into dynamic type
	props, propsDiags := parsePropertiesToDynamic(*foundResource.Properties)
	resp.Diagnostics.Append(propsDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Extract ARN from properties
	propsMap := parsePropertiesToMap(*foundResource.Properties)
	arnValue := extractArn(propsMap)

	data.Exists = types.BoolValue(true)
	data.Properties = props
	if arnValue != "" {
		data.Arn = types.StringValue(arnValue)
	} else {
		data.Arn = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// parsePropertiesToMap parses the JSON properties string into a map.
func parsePropertiesToMap(jsonStr string) map[string]interface{} {
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &props); err != nil {
		return nil
	}
	return props
}

// parsePropertiesToDynamic converts JSON properties to a Terraform dynamic value.
func parsePropertiesToDynamic(jsonStr string) (types.Dynamic, diag.Diagnostics) {
	var diags diag.Diagnostics
	var props map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &props); err != nil {
		diags.AddError("Failed to parse properties", err.Error())
		return types.DynamicNull(), diags
	}

	val, err := convertToAttrValue(props)
	if err != nil {
		diags.AddError("Failed to convert properties", err.Error())
		return types.DynamicNull(), diags
	}

	return types.DynamicValue(val), diags
}

// convertToAttrValue recursively converts Go values to Terraform attr.Value.
func convertToAttrValue(v interface{}) (attr.Value, error) {
	if v == nil {
		return types.StringNull(), nil
	}

	switch val := v.(type) {
	case string:
		return types.StringValue(val), nil
	case float64:
		// JSON numbers are always float64
		return types.Float64Value(val), nil
	case bool:
		return types.BoolValue(val), nil
	case []interface{}:
		if len(val) == 0 {
			return types.ListNull(types.StringType), nil
		}
		elements := make([]attr.Value, len(val))
		for i, elem := range val {
			converted, err := convertToAttrValue(elem)
			if err != nil {
				return nil, err
			}
			elements[i] = converted
		}
		// Determine element type from first element
		elemType := elements[0].Type(context.Background())
		return types.ListValueMust(elemType, elements), nil
	case map[string]interface{}:
		if len(val) == 0 {
			return types.MapNull(types.StringType), nil
		}
		elements := make(map[string]attr.Value)
		for k, elem := range val {
			converted, err := convertToAttrValue(elem)
			if err != nil {
				return nil, err
			}
			elements[k] = converted
		}
		// Use object type for mixed-type maps
		attrTypes := make(map[string]attr.Type)
		for k, v := range elements {
			attrTypes[k] = v.Type(context.Background())
		}
		return types.ObjectValueMust(attrTypes, elements), nil
	default:
		// Fallback to string representation
		return types.StringValue(fmt.Sprintf("%v", val)), nil
	}
}
