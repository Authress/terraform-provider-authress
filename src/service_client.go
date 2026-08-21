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
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

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
	Statements  []AccessRecordStatementResource   `tfsdk:"statement"`
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
				Description: "The unique identifier for the service client. Assigned by Authress on creation.",
				Computed:    true,
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
			"statement": schema.ListNestedBlock{
				Description: "Inline access statements. Upserts an access record with the same ID as the service client, granting it the specified roles on the specified resources.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"roles": schema.ListAttribute{
							Description: "The roles to grant (e.g. Authress:Owner).",
							Required:    true,
							ElementType: TerraformType.StringType,
						},
					},
					Blocks: map[string]schema.Block{
						"resource": schema.ListNestedBlock{
							Description: "Resources the roles apply to.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"resource_uri": schema.StringAttribute{
										Description: "The resource URI pattern (e.g. * for all resources).",
										Required:    true,
									},
								},
							},
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
		detail := "Could not create service client, unexpected error: " + err.Error()
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | body: %s", clientErr.StatusCode(), string(clientErr.Body()))
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to create service client:",
			GetErrorWrapper(detail),
		)
		return
	}


	// Save state immediately after client creation — before key registration.
	// If key creation fails, the next apply will run Update (not Create) and
	// handle key diffing correctly without creating duplicate clients.
	planned.ClientId = TerraformType.StringValue(returnedClient.ClientId)
	planned.CreatedTime = TerraformType.StringValue(returnedClient.CreatedTime.Format(time.RFC3339))
	if returnedClient.HasName() {
		planned.Name = TerraformType.StringValue(returnedClient.GetName())
	}
	diags = resp.State.Set(ctx, planned)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
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
			detail := fmt.Sprintf("Could not create access key with public_key %q: %s", publicKey, keyErr.Error())
			var clientErr *apis.ClientHttpError
			if errors.As(keyErr, &clientErr) && len(clientErr.Body()) > 0 {
				detail += "\nResponse body: " + string(clientErr.Body())
			}
			resp.Diagnostics.AddError("Failed to create access key", GetErrorWrapper(detail))
			return
		}
		planned.AccessKeys[i].KeyId = TerraformType.StringValue(returnedKey.GetKeyId())
	}

	planned.ClientId = TerraformType.StringValue(returnedClient.ClientId)
	planned.CreatedTime = TerraformType.StringValue(returnedClient.CreatedTime.Format(time.RFC3339))
	if returnedClient.HasName() {
		planned.Name = TerraformType.StringValue(returnedClient.GetName())
	}

	// Upsert access record if inline statements are configured
	if len(planned.Statements) > 0 {
		if err := upsertServiceClientAccessRecord(ctx, r.sdk, clientId, planned.Name.ValueString(), planned.Statements); err != nil {
			resp.Diagnostics.AddError("Failed to upsert access record for service client", err.Error())
			return
		}
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
		detail := "Could not read Authress service client ID " + clientId + ": " + err.Error()
		if errors.As(err, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | body: %s", clientErr.StatusCode(), string(clientErr.Body()))
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to get service client:",
			GetErrorWrapper(detail),
		)
		return
	}

	// Preserve inline statements from prior state — the service client API doesn't return them
	previousStatements := current.Statements
	// Access keys are immutable and keyed by the user's config public_key — never overwrite from API
	previousAccessKeys := current.AccessKeys

	// Recover key_id from API if state is corrupted (empty key_id from prior provider version)
	hasEmptyKeyId := false
	for _, k := range previousAccessKeys {
		if k.KeyId.ValueString() == "" && k.PublicKey.ValueString() != "" {
			hasEmptyKeyId = true
			break
		}
	}
	if hasEmptyKeyId && returnedClient.HasVerificationKeys() {
		apiKeys := make(map[string]string)
		for _, vk := range returnedClient.GetVerificationKeys() {
			apiKeys[vk.GetPublicKey()] = vk.GetKeyId()
		}
		for i := range previousAccessKeys {
			if previousAccessKeys[i].KeyId.ValueString() == "" {
				pk := previousAccessKeys[i].PublicKey.ValueString()
				if kid, ok := apiKeys[pk]; ok {
					previousAccessKeys[i].KeyId = TerraformType.StringValue(kid)
				}
			}
		}
	}

	current = mapSdkServiceClientToTerraform(returnedClient)
	current.Statements = previousStatements
	current.AccessKeys = previousAccessKeys
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

	clientId := current.ClientId.ValueString()
	if clientId == "" {
		// State is corrupted from a partial Create (clientId was never populated).
		// Remove from state — the next apply will see the resource as new and run Create.
		resp.State.RemoveResource(ctx)
		resp.Diagnostics.AddWarning(
			"Service client state was corrupted (empty clientId)",
			"The resource has been removed from state. Re-run `tofu apply` to create it.",
		)
		return
	}

	sdkClient := mapTerraformServiceClientToSdk(&planned)
	_, _, err := r.sdk.ServiceClients.UpdateClient(ctx, clientId, sdkClient)
	if err != nil {
		detail := "Could not update service client, unexpected error: " + err.Error()
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | clientId=%q | body: %s", clientErr.StatusCode(), clientId, string(clientErr.Body()))
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to update service client:",
			GetErrorWrapper(detail),
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

	// Track key_id for newly created keys
	newKeyIds := make(map[string]string) // public_key → key_id

	// Keys in plan but not in state → POST (create)
	for publicKey := range plannedKeys {
		if _, exists := existingKeys[publicKey]; !exists {
			accessKeyBody := &models.ClientAccessKey{}
			accessKeyBody.SetPublicKey(publicKey)

			returnedKey, _, keyErr := r.sdk.ServiceClients.RequestAccessKey(ctx, clientId, accessKeyBody)
			if keyErr != nil {
				detail := fmt.Sprintf("Could not create access key with public_key %q: %s", publicKey, keyErr.Error())
				var clientErr *apis.ClientHttpError
				if errors.As(keyErr, &clientErr) && len(clientErr.Body()) > 0 {
					detail += "\nResponse body: " + string(clientErr.Body())
				}
				resp.Diagnostics.AddError("Failed to create access key", detail)
				return
			}
			newKeyIds[publicKey] = returnedKey.GetKeyId()
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

	// Build final access key state: config's public_key + key_id from state or create response
	// If state has empty key_id (corrupted by prior provider version), recover from API
	var recoveredKeyIds map[string]string
	finalKeys := make([]ServiceClientAccessKeyResource, 0, len(planned.AccessKeys))
	for _, planKey := range planned.AccessKeys {
		pk := planKey.PublicKey.ValueString()
		var keyId string
		if existing, ok := existingKeys[pk]; ok {
			keyId = existing.KeyId.ValueString()
		}
		if newId, ok := newKeyIds[pk]; ok {
			keyId = newId
		}
		// Recover from API if key_id is still empty (corrupted state)
		if keyId == "" {
			if recoveredKeyIds == nil {
				recoveredKeyIds = make(map[string]string)
				refreshedForKeys, _, _ := r.sdk.ServiceClients.GetClient(ctx, clientId)
				if refreshedForKeys != nil && refreshedForKeys.HasVerificationKeys() {
					for _, vk := range refreshedForKeys.GetVerificationKeys() {
						recoveredKeyIds[vk.GetPublicKey()] = vk.GetKeyId()
					}
				}
			}
			keyId = recoveredKeyIds[pk]
		}
		finalKeys = append(finalKeys, ServiceClientAccessKeyResource{
			PublicKey: planKey.PublicKey,
			KeyId:     TerraformType.StringValue(keyId),
		})
	}
	planned.AccessKeys = finalKeys

	// Re-read client metadata (name, options, tags) — but NOT access keys
	refreshedClient, _, readErr := r.sdk.ServiceClients.GetClient(ctx, clientId)
	if readErr != nil {
		resp.Diagnostics.AddError(
			"Failed to refresh service client after update",
			readErr.Error(),
		)
		return
	}

	refreshed := mapSdkServiceClientToTerraform(refreshedClient)
	planned.ClientId = refreshed.ClientId
	planned.CreatedTime = refreshed.CreatedTime
	planned.Name = refreshed.Name
	planned.Options = refreshed.Options
	planned.Tags = refreshed.Tags

	// Preserve inline statements from plan (not stored on service client API)
	var plannedFromPlan AuthressServiceClientResource
	diags = req.Plan.Get(ctx, &plannedFromPlan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	planned.Statements = plannedFromPlan.Statements

	// Upsert access record if inline statements are configured
	if len(planned.Statements) > 0 {
		if err := upsertServiceClientAccessRecord(ctx, r.sdk, clientId, planned.Name.ValueString(), planned.Statements); err != nil {
			resp.Diagnostics.AddError("Failed to upsert access record for service client", err.Error())
			return
		}
	}

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

	// Delete the access record first if inline statements were configured
	if len(current.Statements) > 0 {
		_, delRecordErr := r.sdk.AccessRecords.DeleteRecord(ctx, clientId).Execute()
		if delRecordErr != nil {
			// Ignore 404 — record may not exist yet or was already deleted
			var clientErr *apis.ClientHttpError
			if !errors.As(delRecordErr, &clientErr) || clientErr.StatusCode() != 404 {
				resp.Diagnostics.AddError(
					"Failed to delete access record for service client",
					fmt.Sprintf("Could not delete access record %q: %s", clientId, delRecordErr.Error()),
				)
				return
			}
		}
	}

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
		Statements:  []AccessRecordStatementResource{},
	}

	if sdkClient.HasName() {
		tf.Name = TerraformType.StringValue(sdkClient.GetName())
	}

	// Tags: only set if API returns non-empty tags — null in plan must stay null in state
	if sdkClient.Tags != nil && len(sdkClient.Tags) > 0 {
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
	sdkClient := &models.Client{}

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

// upsertServiceClientAccessRecord creates or updates an access record with the same ID as the
// service client, granting the client the roles specified in inline statement blocks.
func upsertServiceClientAccessRecord(ctx context.Context, sdk *authress.AuthressClient, clientId string, clientName string, statements []AccessRecordStatementResource) error {
	sdkStatements := make([]models.Statement, 0, len(statements))
	for _, s := range statements {
		roles := make([]string, 0, len(s.Roles))
		for _, role := range s.Roles {
			roles = append(roles, role.ValueString())
		}

		resources := make([]models.Resource, 0, len(s.Resources))
		for _, res := range s.Resources {
			resources = append(resources, models.Resource{ResourceUri: res.ResourceUri.ValueString()})
		}

		sdkStatements = append(sdkStatements, models.Statement{
			Roles:     roles,
			Resources: resources,
		})
	}

	record := models.AccessRecord{
		Name: clientName + " - Access",
	}
	record.SetRecordId(clientId)
	record.SetUsers([]models.User{{UserId: clientId}})
	record.SetStatements(sdkStatements)

	tflog.Debug(ctx, "Upserting access record for service client", map[string]any{"recordId": clientId})

	// POST to create. On 409 (already exists): GET, compare, adopt-or-error.
	_, _, createErr := sdk.AccessRecords.CreateRecord(ctx).AccessRecord(record).Execute()
	if createErr != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(createErr, &clientErr) && clientErr.StatusCode() == 409 {
			// Record already exists — compare before overwriting
			tflog.Debug(ctx, "Access record already exists, checking for adoption", map[string]any{"recordId": clientId})
			existingRecord, _, getErr := sdk.AccessRecords.GetRecord(ctx, clientId).Execute()
			if getErr != nil {
				return fmt.Errorf("failed to read existing access record %q for adoption: %s", clientId, getErr.Error())
			}

			// Build a planned representation for mismatch comparison
			plannedRecord := &AuthressAccessRecordResource{
				RecordId: TerraformType.StringValue(clientId),
				Name:     TerraformType.StringValue(record.Name),
			}
			plannedRecord.Users = make([]AccessRecordUserResource, 0, len(record.GetUsers()))
			for _, u := range record.GetUsers() {
				plannedRecord.Users = append(plannedRecord.Users, AccessRecordUserResource{
					UserId: TerraformType.StringValue(u.UserId),
				})
			}
			plannedRecord.Statements = statements

			mismatches := collectAccessRecordMismatches(plannedRecord, existingRecord)
			if mismatches != nil {
				return fmt.Errorf("cannot adopt existing access record %q — configuration differs from remote:\n%s",
					clientId, formatMismatches("authress_service_client (inline access record)", clientId, mismatches))
			}

			// Matches — adopted, nothing to do
			tflog.Debug(ctx, "Access record adoption succeeded (inline)", map[string]any{"recordId": clientId})
			return nil
		}
		detail := fmt.Sprintf("Could not create access record %q: %s", clientId, createErr.Error())
		if errors.As(createErr, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | body: %s", clientErr.StatusCode(), string(clientErr.Body()))
		}
		return fmt.Errorf("%s", detail)
	}
	return nil
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
