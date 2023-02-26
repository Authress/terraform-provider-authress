package authress

import (
	"context"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-authress/src/sdk"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &roleResource{}
	_ resource.ResourceWithConfigure   = &roleResource{}
	_ resource.ResourceWithImportState = &roleResource{}
)

// NewRoleResource is a helper function to simplify the provider implementation.
func NewRoleResource() resource.Resource {
	return &roleResource{}
}

// roleResource is the resource implementation.
type roleResource struct {
	client *authress.Client
}

// roleResourceModel maps the resource schema data.
type roleResourceModel struct {
	RoleID       types.String     `tfsdk:"role_id"`
	Items       []roleItemModel `tfsdk:"items"`
	LastUpdated types.String     `tfsdk:"last_updated"`
}

// roleItemModel maps role item data.
type roleItemModel struct {
	Coffee   roleItemCoffeeModel `tfsdk:"coffee"`
	Quantity types.Int64          `tfsdk:"quantity"`
}

// roleItemCoffeeModel maps coffee role item data.
type roleItemCoffeeModel struct {
	Name        types.String  `tfsdk:"name"`
	Teaser      types.String  `tfsdk:"teaser"`
	Description types.String  `tfsdk:"description"`
	Price       types.Float64 `tfsdk:"price"`
	Image       types.String  `tfsdk:"image"`
}

// Metadata returns the data source type name.
func (r *roleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

// Schema defines the schema for the data source.
func (r *roleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an role.",
		Attributes: map[string]schema.Attribute{
			"role_id": schema.StringAttribute{
				Description: "The identifier of the role.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{ stringplanmodifier.UseStateForUnknown() },
			},
			"last_updated": schema.StringAttribute{
				Description: "Timestamp of the last Terraform update of the role.",
				Computed:    true,
			},
			"items": schema.ListNestedAttribute{
				Description: "List of items in the role.",
				Required:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"quantity": schema.Int64Attribute{
							Description: "Count of this item in the role.",
							Required:    true,
						},
						"coffee": schema.SingleNestedAttribute{
							Description: "Coffee item in the role.",
							Required:    true,
							Attributes: map[string]schema.Attribute{
								"id": schema.Int64Attribute{
									Description: "Numeric identifier of the coffee.",
									Required:    true,
								},
								"name": schema.StringAttribute{
									Description: "Product name of the coffee.",
									Computed:    true,
								},
								"teaser": schema.StringAttribute{
									Description: "Fun tagline for the coffee.",
									Computed:    true,
								},
								"description": schema.StringAttribute{
									Description: "Product description of the coffee.",
									Computed:    true,
								},
								"price": schema.Float64Attribute{
									Description: "Suggested cost of the coffee.",
									Computed:    true,
								},
								"image": schema.StringAttribute{
									Description: "URI for an image of the coffee.",
									Computed:    true,
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
func (r *roleResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*authress.Client)
}

// Create creates the resource and sets the initial Terraform state.
func (r *roleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan roleResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	var items []authress.RoleItem
	for _, item := range plan.Items {
		items = append(items, authress.RoleItem{
			Coffee: authress.Coffee{
				ID: int(item.Coffee.ID.ValueInt64()),
			},
			Quantity: int(item.Quantity.ValueInt64()),
		})
	}

	// Create new role
	role, err := r.client.CreateRole(items)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating role",
			"Could not create role, unexpected error: "+err.Error(),
		)
		return
	}

	// Map response body to schema and populate Computed attribute values
	plan.ID = types.StringValue(strconv.Itoa(role.ID))
	for itemIndex, item := range role.Items {
		plan.Items[itemIndex] = roleItemModel{
			Coffee: roleItemCoffeeModel{
				ID:          types.Int64Value(int64(item.Coffee.ID)),
				Name:        types.StringValue(item.Coffee.Name),
				Teaser:      types.StringValue(item.Coffee.Teaser),
				Description: types.StringValue(item.Coffee.Description),
				Price:       types.Float64Value(item.Coffee.Price),
				Image:       types.StringValue(item.Coffee.Image),
			},
			Quantity: types.Int64Value(int64(item.Quantity)),
		}
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *roleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Get current state
	var state roleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get refreshed role value from Authress
	role, err := r.client.GetRole(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Authress Role",
			"Could not read Authress role ID "+state.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Overwrite items with refreshed state
	state.Items = []roleItemModel{}
	for _, item := range role.Items {
		state.Items = append(state.Items, roleItemModel{
			Coffee: roleItemCoffeeModel{
				ID:          types.Int64Value(int64(item.Coffee.ID)),
				Name:        types.StringValue(item.Coffee.Name),
				Teaser:      types.StringValue(item.Coffee.Teaser),
				Description: types.StringValue(item.Coffee.Description),
				Price:       types.Float64Value(item.Coffee.Price),
				Image:       types.StringValue(item.Coffee.Image),
			},
			Quantity: types.Int64Value(int64(item.Quantity)),
		})
	}

	// Set refreshed state
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *roleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan roleResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Generate API request body from plan
	var authressItems []authress.RoleItem
	for _, item := range plan.Items {
		authressItems = append(authressItems, authress.RoleItem{
			Coffee: authress.Coffee{
				ID: int(item.Coffee.ID.ValueInt64()),
			},
			Quantity: int(item.Quantity.ValueInt64()),
		})
	}

	// Update existing role
	_, err := r.client.UpdateRole(plan.ID.ValueString(), authressItems)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Updating Authress Role",
			"Could not update role, unexpected error: "+err.Error(),
		)
		return
	}

	// Fetch updated items from GetRole as UpdateRole items are not
	// populated.
	role, err := r.client.GetRole(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Reading Authress Role",
			"Could not read Authress role ID "+plan.ID.ValueString()+": "+err.Error(),
		)
		return
	}

	// Update resource state with updated items and timestamp
	plan.Items = []roleItemModel{}
	for _, item := range role.Items {
		plan.Items = append(plan.Items, roleItemModel{
			Coffee: roleItemCoffeeModel{
				ID:          types.Int64Value(int64(item.Coffee.ID)),
				Name:        types.StringValue(item.Coffee.Name),
				Teaser:      types.StringValue(item.Coffee.Teaser),
				Description: types.StringValue(item.Coffee.Description),
				Price:       types.Float64Value(item.Coffee.Price),
				Image:       types.StringValue(item.Coffee.Image),
			},
			Quantity: types.Int64Value(int64(item.Quantity)),
		})
	}
	plan.LastUpdated = types.StringValue(time.Now().Format(time.RFC850))

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Delete deletes the resource and removes the Terraform state on success.
func (r *roleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Retrieve values from state
	var state roleResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete existing role
	err := r.client.DeleteRole(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting Authress Role",
			"Could not delete role, unexpected error: "+err.Error(),
		)
		return
	}
}

func (r *roleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Retrieve import ID and save to id attribute
	resource.ImportStatePassthroughID(ctx, path.Root("role_id"), req, resp)
}
