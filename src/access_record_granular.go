package authress

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	authress "github.com/authress/authress-sdk.go"
	"github.com/authress/authress-sdk.go/apis"
	"github.com/authress/authress-sdk.go/models"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &AccessRecordGranularInterfaceProvider{}
	_ resource.ResourceWithConfigure   = &AccessRecordGranularInterfaceProvider{}
	_ resource.ResourceWithImportState = &AccessRecordGranularInterfaceProvider{}
)

func NewAccessRecordGranularResource() resource.Resource {
	return &AccessRecordGranularInterfaceProvider{}
}

type AccessRecordGranularInterfaceProvider struct {
	sdk *authress.AuthressClient
}

/*******************************************/
/* Data stored in Terraform State and Plan */
/*******************************************/
type AuthressAccessRecordGranularResource struct {
	RecordId   TerraformType.String                    `tfsdk:"record_id"`
	Name       TerraformType.String                    `tfsdk:"name"`
	Statements []AccessRecordGranularStatementResource `tfsdk:"statement"`
}

type AccessRecordGranularStatementResource struct {
	Roles     []TerraformType.String         `tfsdk:"roles"`
	Resources []AccessRecordResourceResource `tfsdk:"resource"`
	Users     []AccessRecordUserResource     `tfsdk:"user"`
	Groups    []AccessRecordGroupResource    `tfsdk:"group"`
}

type AccessRecordGroupResource struct {
	GroupId TerraformType.String `tfsdk:"group_id"`
}

/*******************************************/
/*******************************************/

func (r *AccessRecordGranularInterfaceProvider) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_record_granular"
}

func (r *AccessRecordGranularInterfaceProvider) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Authress Access Record with per-statement user and group assignments. Use when different statements target different principals.",
		MarkdownDescription: "Manages an Authress [Access Record](https://authress.io/knowledge-base/docs/category/access-records) with per-statement user and group assignments.",
		Attributes: map[string]schema.Attribute{
			"record_id": schema.StringAttribute{
				Description:   "The unique identifier for the access record.",
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Description: "A helpful name for this access record.",
				Required:    true,
			},
		},
		Blocks: map[string]schema.Block{
			"statement": schema.ListNestedBlock{
				Description: "Access statements with per-statement user/group targeting.",
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
						"user": schema.ListNestedBlock{
							Description: "Users or service clients targeted by this statement.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"user_id": schema.StringAttribute{
										Description: "The user ID or service client ID.",
										Required:    true,
									},
								},
							},
						},
						"group": schema.ListNestedBlock{
							Description: "Groups targeted by this statement.",
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"group_id": schema.StringAttribute{
										Description: "The group ID.",
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

func (r *AccessRecordGranularInterfaceProvider) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.sdk = req.ProviderData.(*authress.AuthressClient)
}

func (r *AccessRecordGranularInterfaceProvider) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var planned AuthressAccessRecordGranularResource
	diags := req.Plan.Get(ctx, &planned)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := planned.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecordGranular.Create starting", map[string]any{"recordId": recordId})

	sdkRecord := mapTerraformAccessRecordGranularToSdk(&planned)

	// POST to create, fall back to adoption on 409 (already exists)
	_, _, err := r.sdk.AccessRecords.CreateRecord(ctx).AccessRecord(sdkRecord).Execute()
	if err != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) && clientErr.StatusCode() == 409 {
			// Record already exists — attempt adoption
			tflog.Debug(ctx, "Access record already exists, attempting adoption", map[string]any{"recordId": recordId})
			existingRecord, _, getErr := r.sdk.AccessRecords.GetRecord(ctx, recordId).Execute()
			if getErr != nil {
				resp.Diagnostics.AddError("Failed to read existing access record for adoption", getErr.Error())
				return
			}

			mismatches := collectAccessRecordGranularMismatches(&planned, existingRecord)
			if mismatches != nil {
				resp.Diagnostics.AddError(
					"Cannot adopt existing access record",
					formatMismatches("authress_access_record_granular", recordId, mismatches),
				)
				return
			}

			// Adopt: state matches — populate from existing
			tflog.Debug(ctx, "Access record adoption succeeded", map[string]any{"recordId": recordId})
			diags = resp.State.Set(ctx, planned)
			resp.Diagnostics.Append(diags...)
			return
		}
		detail := fmt.Sprintf("Could not create access record %q: %s", recordId, err.Error())
		if errors.As(err, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | body: %s", clientErr.StatusCode(), string(clientErr.Body()))
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to create access record:",
			GetErrorWrapper(detail),
		)
		return
	}

	tflog.Debug(ctx, "AccessRecordGranular.Create succeeded", map[string]any{"recordId": recordId})
	diags = resp.State.Set(ctx, planned)
	resp.Diagnostics.Append(diags...)
}

func (r *AccessRecordGranularInterfaceProvider) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var current AuthressAccessRecordGranularResource
	diags := req.State.Get(ctx, &current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := current.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecordGranular.Read starting", map[string]any{"recordId": recordId})

	returnedRecord, _, err := r.sdk.AccessRecords.GetRecord(ctx, recordId).Execute()
	if err != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) && clientErr.StatusCode() == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		detail := fmt.Sprintf("Could not read access record %q: %s", recordId, err.Error())
		if errors.As(err, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | body: %s", clientErr.StatusCode(), string(clientErr.Body()))
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to get access record:",
			GetErrorWrapper(detail),
		)
		return
	}

	current = mapSdkAccessRecordGranularToTerraform(returnedRecord)
	diags = resp.State.Set(ctx, &current)
	resp.Diagnostics.Append(diags...)
}

func (r *AccessRecordGranularInterfaceProvider) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planned AuthressAccessRecordGranularResource
	diags := req.Plan.Get(ctx, &planned)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := planned.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecordGranular.Update starting", map[string]any{"recordId": recordId})

	sdkRecord := mapTerraformAccessRecordGranularToSdk(&planned)

	_, err := r.sdk.AccessRecords.UpdateRecord(ctx, recordId).AccessRecord(sdkRecord).Execute()
	if err != nil {
		detail := fmt.Sprintf("Could not update access record %q: %s", recordId, err.Error())
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | body: %s", clientErr.StatusCode(), string(clientErr.Body()))
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to update access record:",
			GetErrorWrapper(detail),
		)
		return
	}

	tflog.Debug(ctx, "AccessRecordGranular.Update succeeded", map[string]any{"recordId": recordId})
	diags = resp.State.Set(ctx, planned)
	resp.Diagnostics.Append(diags...)
}

func (r *AccessRecordGranularInterfaceProvider) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var current AuthressAccessRecordGranularResource
	diags := req.State.Get(ctx, &current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := current.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecordGranular.Delete starting", map[string]any{"recordId": recordId})

	_, err := r.sdk.AccessRecords.DeleteRecord(ctx, recordId).Execute()
	if err != nil {
		var clientErr *apis.ClientHttpError
		if errors.As(err, &clientErr) && clientErr.StatusCode() == 404 {
			return
		}
		detail := fmt.Sprintf("Could not delete access record %q: %s", recordId, err.Error())
		if errors.As(err, &clientErr) {
			detail += fmt.Sprintf("\nHTTP %d | body: %s", clientErr.StatusCode(), string(clientErr.Body()))
		}
		resp.Diagnostics.AddError(
			"Authress API Response: Attempted to delete access record:",
			GetErrorWrapper(detail),
		)
		return
	}
}

func (r *AccessRecordGranularInterfaceProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("record_id"), req, resp)
}

// ─── SDK Mapping ─────────────────────────────────────────────────────────────

func mapTerraformAccessRecordGranularToSdk(tf *AuthressAccessRecordGranularResource) models.AccessRecord {
	record := models.AccessRecord{
		Name: tf.Name.ValueString(),
	}

	recordId := tf.RecordId.ValueString()
	record.SetRecordId(recordId)

	statements := make([]models.Statement, 0, len(tf.Statements))
	for _, s := range tf.Statements {
		roles := make([]string, 0, len(s.Roles))
		for _, role := range s.Roles {
			roles = append(roles, role.ValueString())
		}

		resources := make([]models.Resource, 0, len(s.Resources))
		for _, res := range s.Resources {
			resources = append(resources, models.Resource{ResourceUri: res.ResourceUri.ValueString()})
		}

		users := make([]models.User, 0, len(s.Users))
		for _, u := range s.Users {
			users = append(users, models.User{UserId: u.UserId.ValueString()})
		}

		groups := make([]models.LinkedGroup, 0, len(s.Groups))
		for _, g := range s.Groups {
			groups = append(groups, models.LinkedGroup{GroupId: g.GroupId.ValueString()})
		}

		stmt := models.Statement{
			Roles:     roles,
			Resources: resources,
		}
		if len(users) > 0 {
			stmt.Users = users
		}
		if len(groups) > 0 {
			stmt.Groups = groups
		}

		statements = append(statements, stmt)
	}
	record.SetStatements(statements)

	return record
}

func mapSdkAccessRecordGranularToTerraform(sdkRecord *models.AccessRecord) AuthressAccessRecordGranularResource {
	tf := AuthressAccessRecordGranularResource{
		RecordId: TerraformType.StringValue(sdkRecord.GetRecordId()),
		Name:     TerraformType.StringValue(sdkRecord.GetName()),
	}

	sdkStatements := sdkRecord.GetStatements()
	tf.Statements = make([]AccessRecordGranularStatementResource, 0, len(sdkStatements))
	for _, s := range sdkStatements {
		roles := make([]TerraformType.String, 0, len(s.Roles))
		for _, role := range s.Roles {
			roles = append(roles, TerraformType.StringValue(role))
		}

		resources := make([]AccessRecordResourceResource, 0, len(s.Resources))
		for _, res := range s.Resources {
			resources = append(resources, AccessRecordResourceResource{
				ResourceUri: TerraformType.StringValue(res.ResourceUri),
			})
		}

		users := make([]AccessRecordUserResource, 0, len(s.Users))
		for _, u := range s.Users {
			users = append(users, AccessRecordUserResource{
				UserId: TerraformType.StringValue(u.UserId),
			})
		}

		groups := make([]AccessRecordGroupResource, 0, len(s.Groups))
		for _, g := range s.Groups {
			groups = append(groups, AccessRecordGroupResource{
				GroupId: TerraformType.StringValue(g.GroupId),
			})
		}

		tf.Statements = append(tf.Statements, AccessRecordGranularStatementResource{
			Roles:     roles,
			Resources: resources,
			Users:     users,
			Groups:    groups,
		})
	}

	return tf
}

// ─── Adoption ────────────────────────────────────────────────────────────────

func collectAccessRecordGranularMismatches(planned *AuthressAccessRecordGranularResource, existing *models.AccessRecord) []FieldMismatch {
	checks := []*FieldMismatch{
		compareField("name", planned.Name.ValueString(), existing.GetName()),
	}

	existingStmts := existing.GetStatements()
	if len(planned.Statements) != len(existingStmts) {
		checks = append(checks, &FieldMismatch{
			Field:    "statements (count)",
			Expected: fmt.Sprintf("%d", len(planned.Statements)),
			Actual:   fmt.Sprintf("%d", len(existingStmts)),
		})
	} else {
		for i, s := range planned.Statements {
			if i >= len(existingStmts) {
				break
			}
			existingStmt := existingStmts[i]

			// Roles
			plannedRoles := make([]string, 0, len(s.Roles))
			for _, r := range s.Roles {
				plannedRoles = append(plannedRoles, r.ValueString())
			}
			if len(plannedRoles) != len(existingStmt.Roles) {
				checks = append(checks, &FieldMismatch{
					Field:    fmt.Sprintf("statements[%d].roles (count)", i),
					Expected: fmt.Sprintf("%d", len(plannedRoles)),
					Actual:   fmt.Sprintf("%d", len(existingStmt.Roles)),
				})
			} else {
				for j, role := range plannedRoles {
					checks = append(checks, compareField(
						fmt.Sprintf("statements[%d].roles[%d]", i, j), role, existingStmt.Roles[j]))
				}
			}

			// Resources
			if len(s.Resources) != len(existingStmt.Resources) {
				checks = append(checks, &FieldMismatch{
					Field:    fmt.Sprintf("statements[%d].resources (count)", i),
					Expected: fmt.Sprintf("%d", len(s.Resources)),
					Actual:   fmt.Sprintf("%d", len(existingStmt.Resources)),
				})
			} else {
				for j, res := range s.Resources {
					checks = append(checks, compareField(
						fmt.Sprintf("statements[%d].resources[%d].resource_uri", i, j),
						res.ResourceUri.ValueString(), existingStmt.Resources[j].ResourceUri))
				}
			}

			// Users
			if len(s.Users) != len(existingStmt.Users) {
				checks = append(checks, &FieldMismatch{
					Field:    fmt.Sprintf("statements[%d].users (count)", i),
					Expected: fmt.Sprintf("%d", len(s.Users)),
					Actual:   fmt.Sprintf("%d", len(existingStmt.Users)),
				})
			} else {
				for j, u := range s.Users {
					if j < len(existingStmt.Users) {
						checks = append(checks, compareField(
							fmt.Sprintf("statements[%d].users[%d].user_id", i, j),
							u.UserId.ValueString(), existingStmt.Users[j].UserId))
					}
				}
			}

			// Groups
			if len(s.Groups) != len(existingStmt.Groups) {
				checks = append(checks, &FieldMismatch{
					Field:    fmt.Sprintf("statements[%d].groups (count)", i),
					Expected: fmt.Sprintf("%d", len(s.Groups)),
					Actual:   fmt.Sprintf("%d", len(existingStmt.Groups)),
				})
			} else {
				for j, g := range s.Groups {
					if j < len(existingStmt.Groups) {
						checks = append(checks, compareField(
							fmt.Sprintf("statements[%d].groups[%d].group_id", i, j),
							g.GroupId.ValueString(), existingStmt.Groups[j].GroupId))
					}
				}
			}
		}
	}

	return collectMismatches(checks...)
}
