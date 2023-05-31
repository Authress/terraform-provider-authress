package authress

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	AuthressSdk "github.com/authress/terraform-provider-authress/src/sdk"
)

// Ensure the implementation satisfies the expected interfaces
var (
	_ provider.Provider = &authressProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New() provider.Provider {
	return &authressProvider{}
}

// authressProvider is the provider implementation.
type authressProvider struct{}

// authressSdkTFModel maps provider schema data to a Go type.
type authressSdkTFModel struct {
	CustomDomain     TerraformType.String `tfsdk:"custom_domain"`
	AccessKey 		 TerraformType.String `tfsdk:"access_key"`
}

// Metadata returns the provider type name.
func (p *authressProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "authress"
}

// Schema defines the provider-level schema for configuration data.
func (p *authressProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Deploy resources to your Authress account.",
		Attributes: map[string]schema.Attribute{
			"custom_domain": schema.StringAttribute{
				Description: "Your Authress custom domain. [Configure a custom domain for Authress account](https://authress.io/app/#/settings?focus=domain) or use the [provided domain](https://authress.io/app/#/api?route=overview).",
				Required: true,
			},
			"access_key": schema.StringAttribute{
				Description: "The access key for the Authress API. Should be [configured by your CI/CD](https://authress.io/knowledge-base/docs/category/cicd) automatically.",
				Optional: 	true,
				Sensitive: 	true,
			},
		},
	}
}

// Configure prepares a Authress API client for data sources and resources.
func (p *authressProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Authress client")

	// Retrieve provider data from configuration
	var config authressSdkTFModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.CustomDomain.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("custom_domain"),
			"Unknown Authress API CustomDomain",
			"Cannot connect to the Authress API as there is an unknown configuration value for the Authress custom_domain. "+
				"Set the value in the provider configuration",
		)
	}

	if config.AccessKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key"),
			"Unknown Authress API Access Key",
			"Cannot connect to the Authress API as there is an unknown configuration value for the Authress access key. "+
				"Set the value by following the OIDC CI/CD guide in the Authress Knowledge Base: https://authress.io/knowledge-base/docs/category/cicd",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	accessKey := os.Getenv("AUTHRESS_KEY")
	var customDomain string

	if !config.CustomDomain.IsNull() {
		customDomain = config.CustomDomain.ValueString()
		if !strings.HasPrefix(customDomain, "http") {
			customDomain = "https://" + customDomain
		}
	}

	if !config.AccessKey.IsNull() {
		accessKey = config.AccessKey.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if customDomain == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("custom_domain"),
			"Missing Authress API CustomDomain",
			"Cannot connect to the Authress API: Missing Authress custom_domain. " +
				"Set the 'custom_domain' value by adding a terraform provider block for authress",
		)
	}

	if accessKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("access_key"),
			"Missing Authress API Access Key",
			"Cannot connect to the Authress API: Missing Authress Access Key. "+
				"Set the 'access_key' value by running the CI/CD Automation https://authress.io/knowledge-base/docs/category/cicd, or adding a terraform provider block for authress",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "authress_custom_domain", customDomain)
	ctx = tflog.SetField(ctx, "authress_access_key", accessKey)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "authress_access_key")

	tflog.Debug(ctx, "Creating Authress client")

	// Create a new Authress client using the configuration values
	client, err := AuthressSdk.NewClient(customDomain, accessKey, GetBuildInfo().Version)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Authress API Client",
			"An unexpected error occurred when creating the Authress API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"Authress Client Error: " + err.Error(),
		)
		return
	}

	// Make the Authress client available during DataSource and Resource type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured Authress client", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *authressProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

// Resources defines the resources implemented in the provider.
func (p *authressProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		// Linked to in the role.go
		NewRoleResource,
	}
}
