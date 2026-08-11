package authress

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"

	authress "github.com/authress/authress-sdk.go"
	"github.com/authress/authress-sdk.go/apis"
	"github.com/authress/authress-sdk.go/models"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &RoleInterfaceProvider{}
	_ resource.ResourceWithConfigure   = &RoleInterfaceProvider{}
	_ resource.ResourceWithImportState = &RoleInterfaceProvider{}
)

// NewRoleResource is a helper function to simplify the provider implementation.
func NewRoleResource() resource.Resource {
	return &RoleInterfaceProvider{}
}

// RoleInterfaceProvider is the resource implementation.
type RoleInterfaceProvider struct {
	sdk *authress.AuthressClient
}

/*******************************************/
/* Data stored in Terraform State and Plan */
/*******************************************/
type AuthressRoleResource struct {
	// Remove after https://developer.hashicorp.com/terraform/plugin/framework/acctests#implement-id-attribute https://github.com/hashicorp/terraform-plugin-sdk/issues/1072
	LegacyID    TerraformType.String                     `tfsdk:"id"`
	RoleID      TerraformType.String                     `tfsdk:"role_id"`
	Name        TerraformType.String                     `tfsdk:"name"`
	Description TerraformType.String                     `tfsdk:"description"`
	LastUpdated TerraformType.String                     `tfsdk:"last_updated"`
	Permissions map[string]AuthressRolePermissionResource `tfsdk:"permissions"`
}

type AuthressRolePermissionResource struct {
	Allow    TerraformType.Bool `tfsdk:"allow"`
	Grant    TerraformType.Bool `tfsdk:"grant"`
	Delegate TerraformType.Bool `tfsdk:"delegate"`
}

/*******************************************/
/*******************************************/

// Metadata returns the data source type name.
func (r *RoleInterfaceProvider) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

// Schema defines the schema for the data source.
func (r *RoleInterfaceProvider) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Authress `Role`. Roles are assigned to `Users` for specific `Resources` using an `Access Record`. `Roles` only contain a list of permissions and should be mapped to your existing User Personas. See Authress KB for more information.",
		MarkdownDescription: "Manages an Authress `Role`. Roles are assigned to `Users` for specific `Resources` using an `Access Record`. `Roles` only contain a list of permissions and should be mapped to your existing User Personas. See [Roles and Permissions](https://authress.io/knowledge-base/docs/authorization/permissions#roles) for more information.",
		Attributes: map[string]schema.Attribute{
			"role_id": schema.StringAttribute{
				Description: "Unique identifier for the role, can be specified on creation, and used by records to map to permissions.",
				Required:    true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^ro_[a-zA-Z0-9-._:@]+$`),
						"must begin with the prefix ro_ and contain only alphanumeric characters and [-._:]",
					),
				},
			},
			"id": schema.StringAttribute{
				Description: "Legacy Terraform property that is not actually used",
				Computed:    true,
			},
			"last_updated": schema.StringAttribute{
				Description: "Timestamp of the last Terraform update of the role.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "A helpful name for this role. The name displays in the Authress Management Portal",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
				},
			},
			"description": schema.StringAttribute{
				Description: "An extended description field that can be used to store additional information about the usage of the role.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(0, 1024),
				},
			},
			"permissions": schema.MapNestedAttribute{
				Description: "A map of the permissions. The key of the map is the action the permission grants, can be scoped using `:` and parent actions imply sub-resource permissions, `action:*` or `action` implies `action:sub-action`. This property is case-insensitive, it will always be cast to lowercase before comparing actions to user permissions.",
				Required:    true,
				Validators: []validator.Map{
					mapvalidator.KeysAre(
						stringvalidator.LengthBetween(1, 64),
						stringvalidator.RegexMatches(
							regexp.MustCompile(`^([*]|[a-zA-Z0-9-_:]+(:[*])?)$`),
							"must contain only alphanumeric characters and colons used as namespace separators",
						),
					),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"allow": schema.BoolAttribute{
							Description:   "Does this permission grant the user the ability to execute the action?",
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.Bool{boolDefault(false)},
						},
						"grant": schema.BoolAttribute{
							Description:   "Allows the user to give the permission to others without being able to execute the action.",
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.Bool{boolDefault(false)},
						},
						"delegate": schema.BoolAttribute{
							Description:   "Allows delegating or granting the permission to others without being able to execute the action.",
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.Bool{boolDefault(false)},
						},
					},
				},
			},
		},
	}
}

// Configure adds the provider configured client to the data source.
func (r *RoleInterfaceProvider) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.sdk = req.ProviderData.(*authress.AuthressClient)
}

// Create creates the resource and sets the initial Terraform state.
func (r *RoleInterfaceProvider) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plannedAuthressRoleResource AuthressRoleResource
	diags := req.Plan.Get(ctx, &plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	sdkRole := MapTerraformRoleToSdk(&plannedAuthressRoleResource)
	returnedRole, _, err := r.sdk.Roles.CreateRole(ctx, sdkRole)
	if err != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) && clientErr.StatusCode() == 409 {
			// Resource already exists — attempt to adopt
			roleId := plannedAuthressRoleResource.RoleID.ValueString()
			existingRole, _, getErr := r.sdk.Roles.GetRole(ctx, roleId)
			if getErr != nil {
				resp.Diagnostics.AddError("Failed to read existing role for adoption", getErr.Error())
				return
			}

			// Compare configurable fields
			mismatches := collectRoleMismatches(&plannedAuthressRoleResource, existingRole)
			if mismatches != nil {
				resp.Diagnostics.AddError(
					"Cannot adopt existing role",
					formatMismatches("authress_role", roleId, mismatches),
				)
				return
			}

			// Adopt: populate state from existing resource
			plannedAuthressRoleResource = MapSdkRoleToTerraform(existingRole)
			plannedAuthressRoleResource.LastUpdated = TerraformType.StringValue(time.Now().Format(time.RFC850))
			diags = resp.State.Set(ctx, plannedAuthressRoleResource)
			resp.Diagnostics.Append(diags...)
			return
		}

		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to create role:",
			GetErrorWrapper("Could not create role, unexpected error: "+err.Error()),
		)
		return
	}

	plannedAuthressRoleResource = MapSdkRoleToTerraform(returnedRole)
	plannedAuthressRoleResource.LastUpdated = TerraformType.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
}

// Read refreshes the Terraform state with the latest data.
func (r *RoleInterfaceProvider) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var currentAuthressRoleResource AuthressRoleResource
	diags := req.State.Get(ctx, &currentAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleId := currentAuthressRoleResource.RoleID.ValueString()
	returnedRole, _, err := r.sdk.Roles.GetRole(ctx, roleId)
	if err != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) && clientErr.StatusCode() == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to get role:",
			GetErrorWrapper("Could not read Authress role ID "+roleId+": "+err.Error()),
		)
		return
	}

	currentAuthressRoleResource = MapSdkRoleToTerraform(returnedRole)
	diags = resp.State.Set(ctx, &currentAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *RoleInterfaceProvider) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plannedAuthressRoleResource AuthressRoleResource
	diags := req.Plan.Get(ctx, &plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleId := plannedAuthressRoleResource.RoleID.ValueString()
	sdkRole := MapTerraformRoleToSdk(&plannedAuthressRoleResource)
	returnedRole, _, err := r.sdk.Roles.UpdateRole(ctx, roleId, sdkRole)
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to update role:",
			GetErrorWrapper("Could not update role, unexpected error: "+err.Error()),
		)
		return
	}

	plannedAuthressRoleResource = MapSdkRoleToTerraform(returnedRole)
	plannedAuthressRoleResource.LastUpdated = TerraformType.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *RoleInterfaceProvider) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var currentAuthressRoleResource AuthressRoleResource
	diags := req.State.Get(ctx, &currentAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleId := currentAuthressRoleResource.RoleID.ValueString()
	_, err := r.sdk.Roles.DeleteRole(ctx, roleId)
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to delete role:",
			GetErrorWrapper("Could not delete role, unexpected error: "+err.Error()),
		)
		return
	}
}

func (r *RoleInterfaceProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("role_id"), req, resp)
}

func MapSdkRoleToTerraform(sdkRole *models.Role) AuthressRoleResource {
	roleId := sdkRole.GetRoleId()
	description := sdkRole.GetDescription()

	terraformRole := AuthressRoleResource{
		RoleID:      TerraformType.StringValue(roleId),
		LegacyID:    TerraformType.StringValue(roleId),
		Name:        TerraformType.StringValue(sdkRole.Name),
		Description: TerraformType.StringValue(description),
		Permissions: make(map[string]AuthressRolePermissionResource),
	}

	for _, perm := range sdkRole.Permissions {
		terraformRole.Permissions[perm.Action] = AuthressRolePermissionResource{
			Allow:    TerraformType.BoolValue(perm.Allow),
			Grant:    TerraformType.BoolValue(perm.Grant),
			Delegate: TerraformType.BoolValue(perm.Delegate),
		}
	}

	return terraformRole
}

func MapTerraformRoleToSdk(terraformRole *AuthressRoleResource) *models.Role {
	roleId := terraformRole.RoleID.ValueString()
	permissions := make([]models.PermissionObject, 0, len(terraformRole.Permissions))
	for key, value := range terraformRole.Permissions {
		permissions = append(permissions, models.PermissionObject{
			Action:   key,
			Allow:    value.Allow.ValueBool(),
			Grant:    value.Grant.ValueBool(),
			Delegate: value.Delegate.ValueBool(),
		})
	}

	sdkRole := &models.Role{
		RoleId:      &roleId,
		Name:        terraformRole.Name.ValueString(),
		Permissions: permissions,
	}

	if !terraformRole.Description.IsNull() && !terraformRole.Description.IsUnknown() {
		sdkRole.SetDescription(terraformRole.Description.ValueString())
	}

	return sdkRole
}

func collectRoleMismatches(planned *AuthressRoleResource, existing *models.Role) []FieldMismatch {
	checks := []*FieldMismatch{
		compareField("name", planned.Name.ValueString(), existing.Name),
	}

	// Compare permissions: build a map from existing role
	existingPerms := make(map[string]models.PermissionObject, len(existing.Permissions))
	for _, p := range existing.Permissions {
		existingPerms[p.Action] = p
	}

	// Check each planned permission exists with matching values
	for action, plannedPerm := range planned.Permissions {
		existingPerm, exists := existingPerms[action]
		if !exists {
			checks = append(checks, &FieldMismatch{
				Field:    "permissions[" + action + "]",
				Expected: "present",
				Actual:   "missing",
			})
			continue
		}
		if plannedPerm.Allow.ValueBool() != existingPerm.Allow {
			checks = append(checks, &FieldMismatch{
				Field:    "permissions[" + action + "].allow",
				Expected: fmt.Sprintf("%t", plannedPerm.Allow.ValueBool()),
				Actual:   fmt.Sprintf("%t", existingPerm.Allow),
			})
		}
		if plannedPerm.Grant.ValueBool() != existingPerm.Grant {
			checks = append(checks, &FieldMismatch{
				Field:    "permissions[" + action + "].grant",
				Expected: fmt.Sprintf("%t", plannedPerm.Grant.ValueBool()),
				Actual:   fmt.Sprintf("%t", existingPerm.Grant),
			})
		}
		if plannedPerm.Delegate.ValueBool() != existingPerm.Delegate {
			checks = append(checks, &FieldMismatch{
				Field:    "permissions[" + action + "].delegate",
				Expected: fmt.Sprintf("%t", plannedPerm.Delegate.ValueBool()),
				Actual:   fmt.Sprintf("%t", existingPerm.Delegate),
			})
		}
	}

	// Check for extra permissions in existing that aren't in planned
	for action := range existingPerms {
		if _, exists := planned.Permissions[action]; !exists {
			checks = append(checks, &FieldMismatch{
				Field:    "permissions[" + action + "]",
				Expected: "missing",
				Actual:   "present",
			})
		}
	}

	return collectMismatches(checks...)
}

func GetErrorWrapper(errorString string) string {
	responseString := errorString
	if strings.Contains(errorString, "invalid character '<' looking for") {
		responseString = "The custom_domain configured is not valid, please review the Authress provider configuration."
	}
	return "\n************************************************************\nError Details:\n\n" +
		responseString +
		"\n************************************************************\n\n"
}
