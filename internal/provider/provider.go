// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure ProbeProvider satisfies various provider interfaces.
var _ provider.Provider = &ProbeProvider{}

// ProbeProvider defines the provider implementation.
type ProbeProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and run locally, and "test" when running acceptance
	// testing.
	version string
}

// ProbeProviderModel describes the provider data model.
type ProbeProviderModel struct {
	LocalStack types.Bool   `tfsdk:"localstack"`
	Endpoint   types.String `tfsdk:"endpoint"`
	Region     types.String `tfsdk:"region"`
}

func (p *ProbeProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "probe"
	resp.Version = p.version
}

func (p *ProbeProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "The probe provider checks whether AWS resources exist without failing when they don't.",
		Attributes: map[string]schema.Attribute{
			"localstack": schema.BoolAttribute{
				Description: "Explicitly enable or disable LocalStack detection. If not set, auto-detects LocalStack at localhost:4566.",
				Optional:    true,
			},
			"endpoint": schema.StringAttribute{
				Description: "Override the AWS endpoint URL. Setting this implies localstack = true.",
				Optional:    true,
			},
			"region": schema.StringAttribute{
				Description: "AWS region. Defaults to AWS_REGION environment variable, then us-east-1.",
				Optional:    true,
			},
		},
	}
}

func (p *ProbeProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data ProbeProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Determine region
	region := "us-east-1"
	if !data.Region.IsNull() {
		region = data.Region.ValueString()
	} else if envRegion := os.Getenv("AWS_REGION"); envRegion != "" {
		region = envRegion
	} else if envRegion := os.Getenv("AWS_DEFAULT_REGION"); envRegion != "" {
		region = envRegion
	}

	// Determine if using LocalStack
	useLocalStack := false
	endpoint := ""

	if !data.Endpoint.IsNull() {
		// Explicit endpoint implies LocalStack
		useLocalStack = true
		endpoint = data.Endpoint.ValueString()
	} else if !data.LocalStack.IsNull() {
		// Explicit LocalStack setting
		useLocalStack = data.LocalStack.ValueBool()
		if useLocalStack {
			endpoint = "http://localhost:4566"
		}
	} else {
		// Auto-detect LocalStack
		detected, detectedEndpoint := detectLocalStack()
		useLocalStack = detected
		endpoint = detectedEndpoint
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to load AWS configuration",
			err.Error(),
		)
		return
	}

	// Configure for LocalStack if needed
	if useLocalStack {
		cfg.BaseEndpoint = aws.String(endpoint)
		// Use dummy credentials if none are configured
		if _, err := cfg.Credentials.Retrieve(ctx); err != nil {
			cfg.Credentials = credentials.NewStaticCredentialsProvider("test", "test", "")
		}
	}

	// Make the AWS config available to data sources
	resp.DataSourceData = cfg
}

func (p *ProbeProvider) Resources(ctx context.Context) []func() resource.Resource {
	return nil
}

func (p *ProbeProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProbeDataSource,
		NewIamPolicySimulationDataSource,
	}
}

// New creates a new provider instance.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &ProbeProvider{
			version: version,
		}
	}
}

// detectLocalStack probes for LocalStack at the default endpoint.
func detectLocalStack() (bool, string) {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get("http://localhost:4566/_localstack/health")
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, "http://localhost:4566"
	}
	return false, ""
}
