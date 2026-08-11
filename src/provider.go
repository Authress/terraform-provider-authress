package authress

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	authress "github.com/authress/authress-sdk.go"
	"github.com/authress/authress-sdk.go/providers/jwt"
)

var _ provider.Provider = &authressProvider{}

func New() provider.Provider {
	return &authressProvider{}
}

type authressProvider struct{}

type authressProviderModel struct {
	AccessKey types.String `tfsdk:"access_key"`
}

func (p *authressProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "authress"
}

func (p *authressProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Deploy resources to your Authress account.",
		Attributes: map[string]schema.Attribute{
			"access_key": schema.StringAttribute{
				Description: "The access key or JWT for the Authress API. Defaults to AUTHRESS_KEY environment variable. Configure via CI/CD OIDC: https://authress.io/knowledge-base/docs/category/cicd",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *authressProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring Authress client")

	var config authressProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve the JWT token: provider block > env var
	jwtToken := os.Getenv("AUTHRESS_KEY")
	if !config.AccessKey.IsNull() && !config.AccessKey.IsUnknown() {
		jwtToken = config.AccessKey.ValueString()
	}

	if jwtToken == "" {
		resp.Diagnostics.AddError(
			"Missing Authress Access Key",
			"Set the 'access_key' in the provider block or the AUTHRESS_KEY environment variable. "+
				"Configure via CI/CD OIDC: https://authress.io/knowledge-base/docs/category/cicd",
		)
		return
	}

	// Decode the JWT payload (no signature verification) to extract aud claim
	apiUrl, err := extractApiUrlFromJwt(jwtToken)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid Authress Token",
			err.Error(),
		)
		return
	}

	// Construct the SDK client
	parsedUrl, err := url.Parse(apiUrl)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid API URL",
			fmt.Sprintf("Failed to parse API URL from token aud claim: %s", err.Error()),
		)
		return
	}

	tokenProvider := jwt.NewTokenProvider(jwtToken)
	settings := authress.AuthressSettings{
		AuthressApiUrl: parsedUrl,
	}
	client := authress.NewAuthressClient(settings, tokenProvider)

	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured Authress client", map[string]any{"api_url": apiUrl})
}

func (p *authressProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *authressProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewRoleResource,
		NewServiceClientResource,
	}
}

// extractApiUrlFromJwt decodes the JWT payload and extracts the API URL from the aud claim.
// The aud claim must match: https://{accountId}.api.authress.* (any TLD)
func extractApiUrlFromJwt(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return "", fmt.Errorf("token is not a valid JWT (expected 3 dot-separated parts)")
	}

	// Base64url decode the payload (part 1)
	payload := parts[1]
	// Add padding if needed
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}

	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		return "", fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return "", fmt.Errorf("failed to parse JWT payload: %w", err)
	}

	aud, ok := claims["aud"].(string)
	if !ok || aud == "" {
		return "", fmt.Errorf("JWT missing 'aud' claim")
	}

	// Validate aud matches https://{accountId}.api.authress.* pattern
	audPattern := regexp.MustCompile(`^https://[^.]+\.api\.authress\.[^/]+$`)
	if !audPattern.MatchString(aud) {
		return "", fmt.Errorf(
			"the 'aud' claim in your token must match 'https://{accountId}.api.authress.io'. "+
				"Got: %s. Update your GitHub/GitLab OIDC token audience configuration.", aud)
	}

	return aud, nil
}
