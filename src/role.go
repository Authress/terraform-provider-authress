package authress

import (
	"context"
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

	AuthressSdk "terraform-provider-authress/src/sdk"
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
	client *AuthressSdk.Client
}

/*******************************************/
/* Data stored in Terraform State and Plan */
/*******************************************/
type AuthressRoleResource struct {
	// Remove after https://developer.hashicorp.com/terraform/plugin/framework/acctests#implement-id-attribute https://github.com/hashicorp/terraform-plugin-sdk/issues/1072
	LegacyID	TerraformType.String						`tfsdk:"id"`
	RoleID		TerraformType.String						`tfsdk:"role_id"`
	Name 		TerraformType.String						`tfsdk:"name"`
	Description TerraformType.String						`tfsdk:"description"`
	LastUpdated TerraformType.String  						`tfsdk:"last_updated"`
	Permissions map[string]AuthressRolePermissionResource	`tfsdk:"permissions"`
}

type AuthressRolePermissionResource struct {
	Allow 		TerraformType.Bool	`tfsdk:"allow"`
	Grant		TerraformType.Bool	`tfsdk:"grant"`
	Delegate	TerraformType.Bool	`tfsdk:"delegate"`
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
		Description: "Manages an Authress `Role`. Roles are assigned to `Users` for specific `Resources` using an `Access Record`. `Roles` only contain a list of permissions and should be mapped to your existing User Personas. See Authress KB for more information.",
		MarkdownDescription: "Manages an Authress `Role`. Roles are assigned to `Users` for specific `Resources` using an `Access Record`. `Roles` only contain a list of permissions and should be mapped to your existing User Personas. See [Roles and Permissions](https://authress.io/knowledge-base/docs/authorization/permissions#roles) for more information.",
		Attributes: map[string]schema.Attribute {
			"role_id": schema.StringAttribute {
				Description: "Unique identifier for the role, can be specified on creation, and used by records to map to permissions.",
				Required:    true,
				// https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification#requiresreplace
				PlanModifiers: []planmodifier.String{ stringplanmodifier.RequiresReplace() },
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-zA-Z0-9-._:@]+$`),
						"must contain only alphanumeric characters and [-._:]",
					),
				},
			},
			"id": schema.StringAttribute {
				Description: "Legacy Terraform property that is not actually used",
				Computed:    true,
			},
			"last_updated": schema.StringAttribute {
				Description: "Timestamp of the last Terraform update of the role.",
				Computed:    true,
			},
			"name": schema.StringAttribute {
				Description: "A helpful name for this role. The name displays in the Authress Management Portal",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
				},
			},
			"description": schema.StringAttribute {
				Description:	"An extended description field that can be used to store additional information about the usage of the role.",
				Optional:	    true,
				Computed: 		true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(0, 1024),
				},
			},
			"permissions": schema.MapNestedAttribute {
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
					Attributes: map[string]schema.Attribute {
						"allow": schema.BoolAttribute {
							Description: "Does this permission grant the user the ability to execute the action?",
							Required:    true,
						},
						"grant": schema.BoolAttribute {
							Description:	"Allows the user to give the permission to others without being able to execute the action.",
							Optional:   	true,
							Computed: 		true,
						},
						"delegate": schema.BoolAttribute {
							Description: 	"Allows delegating or granting the permission to others without being able to execute the action.",
							Optional:    	true,
							Computed: 		true,
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

	r.client = req.ProviderData.(*AuthressSdk.Client)
}

// Create creates the resource and sets the initial Terraform state.
func (r *RoleInterfaceProvider) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plannedAuthressRoleResource AuthressRoleResource
	diags := req.Plan.Get(ctx, &plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create new role
	authressSdkRole := MapTerraformRoleToSdk(&plannedAuthressRoleResource)
	returnedRole, err := r.client.CreateRole(authressSdkRole)
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to create role:",
			GetErrorWrapper("Could not create role, unexpected error: " + err.Error()),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plannedAuthressRoleResource = MapSdkRoleToTerraform(returnedRole)
	plannedAuthressRoleResource.LastUpdated = TerraformType.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *RoleInterfaceProvider) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var currentAuthressRoleResource AuthressRoleResource
	diags := req.State.Get(ctx, &currentAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed role value from Authress
	authressSdkRole, err := r.client.GetRole(currentAuthressRoleResource.RoleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to get role:",
			GetErrorWrapper("Could not read Authress role ID " + currentAuthressRoleResource.RoleID.ValueString() + ": " + err.Error()),
		)
		return
	}

	// Set refreshed currentAuthressRoleResource
	currentAuthressRoleResource = MapSdkRoleToTerraform(authressSdkRole)
	diags = resp.State.Set(ctx, &currentAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *RoleInterfaceProvider) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plannedAuthressRoleResource AuthressRoleResource
	diags := req.Plan.Get(ctx, &plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plannedAuthressRoleResource
	authressSdkRole := MapTerraformRoleToSdk(&plannedAuthressRoleResource)

	// Update existing role
	returnedRole, err := r.client.UpdateRole(plannedAuthressRoleResource.RoleID.ValueString(), authressSdkRole)
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to update role:",
			GetErrorWrapper("Could not update role, unexpected error: " + err.Error()),
		)
		return
	}

	plannedAuthressRoleResource = MapSdkRoleToTerraform(returnedRole)
	plannedAuthressRoleResource.LastUpdated = TerraformType.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plannedAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *RoleInterfaceProvider) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var currentAuthressRoleResource AuthressRoleResource
	diags := req.State.Get(ctx, &currentAuthressRoleResource)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing role
	err := r.client.DeleteRole(currentAuthressRoleResource.RoleID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to delete role:",
			GetErrorWrapper("Could not delete role, unexpected error: " + err.Error()),
		)
		return
	}
}

func (r *RoleInterfaceProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("role_id"), req, resp)
}

func MapSdkRoleToTerraform(authressSdkRole *AuthressSdk.Role) (AuthressRoleResource) {
	terraformRole := AuthressRoleResource {
		RoleID: TerraformType.StringValue(authressSdkRole.RoleID),
		LegacyID: TerraformType.StringValue(authressSdkRole.RoleID),
		Name: TerraformType.StringValue(authressSdkRole.Name),
		Description: TerraformType.StringValue(authressSdkRole.Description),
		Permissions: make(map[string]AuthressRolePermissionResource),
	}

	for _, authressRolePermission := range authressSdkRole.Permissions {
		terraformRole.Permissions[authressRolePermission.Action] = AuthressRolePermissionResource {
			Allow: TerraformType.BoolValue(authressRolePermission.Allow),
			Grant: TerraformType.BoolValue(authressRolePermission.Grant),
			Delegate: TerraformType.BoolValue(authressRolePermission.Delegate),
		}
   }

   return terraformRole
}

func MapTerraformRoleToSdk(terraformRole *AuthressRoleResource) (AuthressSdk.Role) {
	authressSdkRole := AuthressSdk.Role {
		RoleID: terraformRole.RoleID.ValueString(),
		Name: terraformRole.Name.ValueString(),
		Description: terraformRole.Description.ValueString(),
		Permissions: make([]AuthressSdk.Permission, 0, len(terraformRole.Permissions)),
	}
	for key, value := range terraformRole.Permissions {
		authressSdkRolePermissions := AuthressSdk.Permission {
			Action: key,
			Allow: value.Allow.ValueBool(),
			Grant: value.Grant.ValueBool(),
			Delegate: value.Delegate.ValueBool(),
		}
		authressSdkRole.Permissions = append(authressSdkRole.Permissions, authressSdkRolePermissions)
	}

   return authressSdkRole
}
func GetErrorWrapper(errorString string) (string) {
	responseString := errorString
	if (strings.Contains(errorString, "invalid character '<' looking for")) {
		responseString = "The custom_domain configured is not valid, please review the Authress provider configuration."
	}
	return "\n************************************************************\nError Details:\n\n" +
	responseString +
	"\n************************************************************\n\n"
}