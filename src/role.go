package authress

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

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
	RoleID		types.String	`tfsdk:"role_id"`
	Name 		types.String	`tfsdk:"name"`
	Description types.String	`tfsdk:"description"`
	LastUpdated types.String  	`tfsdk:"last_updated"`
	Permissions types.Map		`tfsdk:"permissions"`
}

type AuthressRolePermissionResource struct {
	Allow 		types.Bool	`tfsdk:"allow"`
	Grant		types.Bool	`tfsdk:"grant"`
	Delegate	types.Bool	`tfsdk:"delegate"`
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
		Description: "Manages an role.",
		Attributes: map[string]schema.Attribute {
			"role_id": schema.StringAttribute {
				Description: "Unique identifier for the role, can be specified on creation, and used by records to map to permissions.",
				Required:    true,
				// https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification#requiresreplace
				PlanModifiers: []planmodifier.String{ stringplanmodifier.RequiresReplace() },
			},
			"last_updated": schema.StringAttribute {
				Description: "Timestamp of the last Terraform update of the role.",
				Computed:    true,
			},
			"name": schema.StringAttribute {
				Description: "A helpful name for this role.",
				Optional:    true,
			},
			"description": schema.StringAttribute {
				Description: "A description for when to the user as well as additional information.",
				Optional:    true,
			},
			"permissions": schema.MapNestedAttribute {
				Description: "A map of the permissions. The key of the map is the action the permission grants, can be scoped using `:` and parent actions imply sub-resource permissions, `action:*` or 8action` implies `action:sub-action`. This property is case-insensitive, it will always be cast to lowercase before comparing actions to user permissions.",
				Required:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute {
						"allow": schema.BoolAttribute {
							Description: "Does this permission grant the user the ability to execute the action?",
							Optional:    true,
						},
						"grant": schema.BoolAttribute {
							Description: "Allows the user to give the permission to others without being able to execute the action.",
							Optional:    true,
						},
						"delegate": schema.BoolAttribute {
							Description: "Allows delegating or granting the permission to others without being able to execute tha action.",
							Optional:    true,
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

	// Generate API request body from plan
	authressSdkRole := AuthressSdk.Role {
		RoleID: plannedAuthressRoleResource.RoleID.ValueString(),
		Name: plannedAuthressRoleResource.Name.ValueString(),
		Description: plannedAuthressRoleResource.Description.ValueString(),
		Permissions: make([]AuthressSdk.Permission, 0, len(plannedAuthressRoleResource.Permissions.Elements())),
	}
	for key, value := range plannedAuthressRoleResource.Permissions {
		authressSdkRolePermissions := AuthressSdk.Permission {
			Action: key,
			Allow: value.Allow.ValueBool(),
			Grant: value.Grant.ValueBool(),
			Delegate: value.Delegate.ValueBool(),
		}
		authressSdkRole.Permissions = append(authressSdkRole.Permissions, authressSdkRolePermissions)
	}

	// Create new role
	_, err := r.client.CreateRole(authressSdkRole)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			"Could not create role, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plannedAuthressRoleResource.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

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
			"Error Reading Authress Role",
			"Could not read Authress role ID " + currentAuthressRoleResource.RoleID.ValueString() + ": " + err.Error(),
		)
		return
	}

	currentAuthressRoleResource.RoleID = types.StringValue(authressSdkRole.RoleID)
	currentAuthressRoleResource.Name = types.StringValue(authressSdkRole.Name)
	currentAuthressRoleResource.Description = types.StringValue(authressSdkRole.Description)
	currentAuthressRoleResource.Permissions = types.MapValue(types.Map, map[string]AuthressRolePermissionResource)

	for _, authressRolePermission := range authressSdkRole.Permissions {
		currentAuthressRoleResource.Permissions[authressRolePermission.Action] = AuthressRolePermissionResource {
			Allow: types.BoolValue(authressRolePermission.Allow),
			Grant: types.BoolValue(authressRolePermission.Grant),
			Delegate: types.BoolValue(authressRolePermission.Delegate),
		}
   }

	// Set refreshed currentAuthressRoleResource
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
	authressSdkRole := AuthressSdk.Role {
		RoleID: plannedAuthressRoleResource.RoleID.ValueString(),
		Name: plannedAuthressRoleResource.Name.ValueString(),
		Description: plannedAuthressRoleResource.Description.ValueString(),
		Permissions: make([]AuthressSdk.Permission, 0, len(plannedAuthressRoleResource.Permissions.Elements())),
	}
	for key, value := range plannedAuthressRoleResource.Permissions.Elements() {
		authressSdkRolePermissions := AuthressSdk.Permission {
			Action: key,
			Allow: value.Allow.BoolString(),
			Grant: value.Grant.BoolString(),
			Delegate: value.Delegate.BoolString(),
		}
		authressSdkRole.Permissions = append(authressSdkRole.Permissions, authressSdkRolePermissions)
	}

	// Update existing role
	_, err := r.client.UpdateRole(plannedAuthressRoleResource.RoleID.ValueString(), authressSdkRole)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Authress Role",
			"Could not update role, unexpected error: "+err.Error(),
		)
		return
	}

	plannedAuthressRoleResource.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

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
			"Error Deleting Authress Role",
			"Could not delete role, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *RoleInterfaceProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("role_id"), req, resp)
}
