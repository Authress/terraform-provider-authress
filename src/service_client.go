package authress

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"

	authress "github.com/authress/authress-sdk.go"
	"github.com/authress/authress-sdk.go/apis"
	"github.com/authress/authress-sdk.go/models"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &ServiceClientInterfaceProvider{}
	_ resource.ResourceWithConfigure   = &ServiceClientInterfaceProvider{}
	_ resource.ResourceWithImportState = &ServiceClientInterfaceProvider{}
)

// NewServiceClientResource is a helper function to simplify the provider implementation.
func NewServiceClientResource() resource.Resource {
	return &ServiceClientInterfaceProvider{}
}

// ServiceClientInterfaceProvider is the resource implementation.
type ServiceClientInterfaceProvider struct {
	sdk *authress.AuthressClient
}

/*******************************************/
/* Data stored in Terraform State and Plan */
/*******************************************/
type AuthressServiceClientResource struct {
	ClientId    TerraformType.String              `tfsdk:"client_id"`
	Name        TerraformType.String              `tfsdk:"name"`
	Tags        map[string]TerraformType.String   `tfsdk:"tags"`
	CreatedTime TerraformType.String              `tfsdk:"created_time"`
	Options     *ServiceClientOptionsResource     `tfsdk:"options"`
	AccessKeys  []ServiceClientAccessKeyResource  `tfsdk:"access_key"`
}

type ServiceClientOptionsResource struct {
	GrantUserPermissionsAccess TerraformType.Bool `tfsdk:"grant_user_permissions_access"`
	GrantTokenGeneration       TerraformType.Bool `tfsdk:"grant_token_generation"`
}

type ServiceClientAccessKeyResource struct {
	PublicKey TerraformType.String `tfsdk:"public_key"`
	KeyId     TerraformType.String `tfsdk:"key_id"`
}

/*******************************************/
/*******************************************/

// Metadata returns the data source type name.
func (r *ServiceClientInterfaceProvider) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_client"
}

// Schema defines the schema for the data source.
func (r *ServiceClientInterfaceProvider) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Authress Service Client. Service Clients are machine identities used for service-to-service authorization.",
		MarkdownDescription: "Manages an Authress [Service Client](https://authress.io/knowledge-base/docs/category/service-clients). Service Clients are machine identities used for service-to-service authorization.",
		Attributes: map[string]schema.Attribute{
			"client_id": schema.StringAttribute{
				Description: "The unique identifier for the service client.",
				Required:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "The display name for the service client.",
				Required:    true,
			},
			"tags": schema.MapAttribute{
				Description: "Arbitrary key-value tags associated with the service client.",
				Optional:    true,
				ElementType: TerraformType.StringType,
			},
			"created_time": schema.StringAttribute{
				Description: "The timestamp when the service client was created.",
				Computed:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"options": schema.SingleNestedBlock{
				Description: "Configuration options for the service client.",
				Attributes: map[string]schema.Attribute{
					"grant_user_permissions_access": schema.BoolAttribute{
						Description:   "Grant the client access to verify authorization on behalf of any user.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolDefault(false)},
					},
					"grant_token_generation": schema.BoolAttribute{
						Description:   "Grant the client access to generate oauth tokens on behalf of the Authress account.",
						Optional:      true,
						Computed:      true,
						PlanModifiers: []planmodifier.Bool{boolDefault(false)},
					},
				},
			},
			"access_key": schema.SetNestedBlock{
				Description: "Access key configuration for the service client. Each key is identified by its public_key.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"public_key": schema.StringAttribute{
							Description: "The Ed25519 public key. Used as the immutable identity for diff matching.",
							Required:    true,
						},
						"key_id": schema.StringAttribute{
							Description: "The key ID assigned by Authress.",
							Computed:    true,
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (r *ServiceClientInterfaceProvider) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.sdk = req.ProviderData.(*authress.AuthressClient)
}

// Create creates the resource and sets the initial Terraform state.
func (r *ServiceClientInterfaceProvider) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var planned AuthressServiceClientResource
	diags := req.Plan.Get(ctx, &planned)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sdkClient := mapTerraformServiceClientToSdk(&planned)
	returnedClient, _, err := r.sdk.ServiceClients.CreateClient(ctx, sdkClient)
	if err != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) && clientErr.StatusCode() == 409 {
			// Resource already exists — attempt to adopt
			clientId := planned.ClientId.ValueString()
			existingClient, _, getErr := r.sdk.ServiceClients.GetClient(ctx, clientId)
			if getErr != nil {
				resp.Diagnostics.AddError("Failed to read existing service client for adoption", getErr.Error())
				return
			}

			mismatches := collectServiceClientMismatches(&planned, existingClient)
			if mismatches != nil {
				resp.Diagnostics.AddError(
					"Cannot adopt existing service client",
					formatMismatches("authress_service_client", clientId, mismatches),
				)
				return
			}

			// Adopt: populate state from existing resource
			planned = mapSdkServiceClientToTerraform(existingClient)
			diags = resp.State.Set(ctx, planned)
			resp.Diagnostics.Append(diags...)
			return
		}

		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to create service client:",
			GetErrorWrapper("Could not create service client, unexpected error: "+err.Error()),
		)
		return
	}

	// After creation, register each access key
	clientId := returnedClient.ClientId
	for i, key := range planned.AccessKeys {
		publicKey := key.PublicKey.ValueString()
		accessKeyBody := &models.ClientAccessKey{}
		accessKeyBody.SetPublicKey(publicKey)

		returnedKey, _, keyErr := r.sdk.ServiceClients.RequestAccessKey(ctx, clientId, accessKeyBody)
		if keyErr != nil {
			resp.Diagnostics.AddError(
				"Failed to create access key",
				fmt.Sprintf("Could not create access key with public_key %q: %s", publicKey, keyErr.Error()),
			)
			return
		}
		planned.AccessKeys[i].KeyId = TerraformType.StringValue(returnedKey.GetKeyId())
	}

	planned.ClientId = TerraformType.StringValue(returnedClient.ClientId)
	planned.CreatedTime = TerraformType.StringValue(returnedClient.CreatedTime.Format(time.RFC3339))
	if returnedClient.HasName() {
		planned.Name = TerraformType.StringValue(returnedClient.GetName())
	}

	diags = resp.State.Set(ctx, planned)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *ServiceClientInterfaceProvider) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var current AuthressServiceClientResource
	diags := req.State.Get(ctx, &current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientId := current.ClientId.ValueString()
	returnedClient, _, err := r.sdk.ServiceClients.GetClient(ctx, clientId)
	if err != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) && clientErr.StatusCode() == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to get service client:",
			GetErrorWrapper("Could not read Authress service client ID "+clientId+": "+err.Error()),
		)
		return
	}

	current = mapSdkServiceClientToTerraform(returnedClient)
	diags = resp.State.Set(ctx, &current)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *ServiceClientInterfaceProvider) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planned AuthressServiceClientResource
	diags := req.Plan.Get(ctx, &planned)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var current AuthressServiceClientResource
	diags = req.State.Get(ctx, &current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientId := planned.ClientId.ValueString()
	sdkClient := mapTerraformServiceClientToSdk(&planned)
	_, _, err := r.sdk.ServiceClients.UpdateClient(ctx, clientId, sdkClient)
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to update service client:",
			GetErrorWrapper("Could not update service client, unexpected error: "+err.Error()),
		)
		return
	}

	// Diff access_key blocks by public_key
	existingKeys := make(map[string]ServiceClientAccessKeyResource, len(current.AccessKeys))
	for _, key := range current.AccessKeys {
		existingKeys[key.PublicKey.ValueString()] = key
	}

	plannedKeys := make(map[string]ServiceClientAccessKeyResource, len(planned.AccessKeys))
	for _, key := range planned.AccessKeys {
		plannedKeys[key.PublicKey.ValueString()] = key
	}

	// Keys in plan but not in state → POST (create)
	for publicKey := range plannedKeys {
		if _, exists := existingKeys[publicKey]; !exists {
			accessKeyBody := &models.ClientAccessKey{}
			accessKeyBody.SetPublicKey(publicKey)

			_, _, keyErr := r.sdk.ServiceClients.RequestAccessKey(ctx, clientId, accessKeyBody)
			if keyErr != nil {
				resp.Diagnostics.AddError(
					"Failed to create access key",
					fmt.Sprintf("Could not create access key with public_key %q: %s", publicKey, keyErr.Error()),
				)
				return
			}
		}
	}

	// Keys in state but not in plan → DELETE
	for publicKey, key := range existingKeys {
		if _, exists := plannedKeys[publicKey]; !exists {
			keyId := key.KeyId.ValueString()
			if keyId == "" {
				continue
			}
			_, keyErr := r.sdk.ServiceClients.DeleteAccessKey(ctx, clientId, keyId)
			if keyErr != nil {
				resp.Diagnostics.AddError(
					"Failed to delete access key",
					fmt.Sprintf("Could not delete access key %q: %s", keyId, keyErr.Error()),
				)
				return
			}
		}
	}

	// Re-read to get accurate state including key_id assignments
	refreshedClient, _, readErr := r.sdk.ServiceClients.GetClient(ctx, clientId)
	if readErr != nil {
		resp.Diagnostics.AddError(
			"Failed to refresh service client after update",
			readErr.Error(),
		)
		return
	}

	planned = mapSdkServiceClientToTerraform(refreshedClient)
	diags = resp.State.Set(ctx, planned)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *ServiceClientInterfaceProvider) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var current AuthressServiceClientResource
	diags := req.State.Get(ctx, &current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	clientId := current.ClientId.ValueString()
	_, err := r.sdk.ServiceClients.DeleteClient(ctx, clientId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to delete service client:",
			GetErrorWrapper("Could not delete service client, unexpected error: "+err.Error()),
		)
		return
	}
}

func (r *ServiceClientInterfaceProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("client_id"), req, resp)
}

func mapSdkServiceClientToTerraform(sdkClient *models.Client) AuthressServiceClientResource {
	tf := AuthressServiceClientResource{
		ClientId:    TerraformType.StringValue(sdkClient.ClientId),
		CreatedTime: TerraformType.StringValue(sdkClient.CreatedTime.Format(time.RFC3339)),
	}

	if sdkClient.HasName() {
		tf.Name = TerraformType.StringValue(sdkClient.GetName())
	}

	if sdkClient.Tags != nil {
		tf.Tags = make(map[string]TerraformType.String, len(sdkClient.Tags))
		for k, v := range sdkClient.Tags {
			tf.Tags[k] = TerraformType.StringValue(v)
		}
	}

	if sdkClient.HasOptions() {
		opts := sdkClient.GetOptions()
		tf.Options = &ServiceClientOptionsResource{
			GrantUserPermissionsAccess: TerraformType.BoolValue(opts.GetGrantUserPermissionsAccess()),
			GrantTokenGeneration:       TerraformType.BoolValue(opts.GetGrantTokenGeneration()),
		}
	}

	if sdkClient.HasVerificationKeys() {
		keys := sdkClient.GetVerificationKeys()
		tf.AccessKeys = make([]ServiceClientAccessKeyResource, 0, len(keys))
		for _, key := range keys {
			tf.AccessKeys = append(tf.AccessKeys, ServiceClientAccessKeyResource{
				PublicKey: TerraformType.StringValue(key.GetPublicKey()),
				KeyId:     TerraformType.StringValue(key.GetKeyId()),
			})
		}
	}

	return tf
}

func mapTerraformServiceClientToSdk(tf *AuthressServiceClientResource) *models.Client {
	sdkClient := &models.Client{
		ClientId: tf.ClientId.ValueString(),
	}

	sdkClient.SetName(tf.Name.ValueString())

	if tf.Tags != nil {
		tags := make(map[string]string, len(tf.Tags))
		for k, v := range tf.Tags {
			tags[k] = v.ValueString()
		}
		sdkClient.SetTags(tags)
	}

	if tf.Options != nil {
		opts := models.NewClientOptions()
		opts.SetGrantUserPermissionsAccess(tf.Options.GrantUserPermissionsAccess.ValueBool())
		opts.SetGrantTokenGeneration(tf.Options.GrantTokenGeneration.ValueBool())
		sdkClient.SetOptions(*opts)
	}

	return sdkClient
}

func collectServiceClientMismatches(planned *AuthressServiceClientResource, existing *models.Client) []FieldMismatch {
	checks := []*FieldMismatch{
		compareField("name", planned.Name.ValueString(), existing.GetName()),
	}

	// Compare tags
	if planned.Tags != nil {
		existingTags := existing.GetTags()
		for k, v := range planned.Tags {
			actual, ok := existingTags[k]
			if !ok {
				checks = append(checks, &FieldMismatch{Field: "tags[" + k + "]", Expected: v.ValueString(), Actual: "(missing)"})
			} else if v.ValueString() != actual {
				checks = append(checks, &FieldMismatch{Field: "tags[" + k + "]", Expected: v.ValueString(), Actual: actual})
			}
		}
		for k, v := range existingTags {
			if _, ok := planned.Tags[k]; !ok {
				checks = append(checks, &FieldMismatch{Field: "tags[" + k + "]", Expected: "(missing)", Actual: v})
			}
		}
	}

	return collectMismatches(checks...)
}
