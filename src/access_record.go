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
	_ resource.Resource                = &AccessRecordInterfaceProvider{}
	_ resource.ResourceWithConfigure   = &AccessRecordInterfaceProvider{}
	_ resource.ResourceWithImportState = &AccessRecordInterfaceProvider{}
)

func NewAccessRecordResource() resource.Resource {
	return &AccessRecordInterfaceProvider{}
}

type AccessRecordInterfaceProvider struct {
	sdk *authress.AuthressClient
}

/*******************************************/
/* Data stored in Terraform State and Plan */
/*******************************************/
type AuthressAccessRecordResource struct {
	RecordId   TerraformType.String            `tfsdk:"record_id"`
	Name       TerraformType.String            `tfsdk:"name"`
	Users      []AccessRecordUserResource      `tfsdk:"user"`
	Statements []AccessRecordStatementResource `tfsdk:"statement"`
}

type AccessRecordUserResource struct {
	UserId TerraformType.String `tfsdk:"user_id"`
}

type AccessRecordStatementResource struct {
	Roles     []TerraformType.String         `tfsdk:"roles"`
	Resources []AccessRecordResourceResource `tfsdk:"resource"`
}

type AccessRecordResourceResource struct {
	ResourceUri TerraformType.String `tfsdk:"resource_uri"`
}

/*******************************************/
/*******************************************/

func (r *AccessRecordInterfaceProvider) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_record"
}

func (r *AccessRecordInterfaceProvider) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Manages an Authress Access Record. Grants users permissions on resources via role assignments.",
		MarkdownDescription: "Manages an Authress [Access Record](https://authress.io/knowledge-base/docs/category/access-records). Grants users permissions on resources via role assignments.",
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
			"user": schema.ListNestedBlock{
				Description: "Users or service clients granted access by this record.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"user_id": schema.StringAttribute{
							Description: "The user ID or service client ID.",
							Required:    true,
						},
					},
				},
			},
			"statement": schema.ListNestedBlock{
				Description: "Access statements defining which roles are granted on which resources.",
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

func (r *AccessRecordInterfaceProvider) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.sdk = req.ProviderData.(*authress.AuthressClient)
}

func (r *AccessRecordInterfaceProvider) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var planned AuthressAccessRecordResource
	diags := req.Plan.Get(ctx, &planned)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := planned.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecord.Create starting", map[string]any{"recordId": recordId})

	sdkRecord := mapTerraformAccessRecordToSdk(&planned)

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

			mismatches := collectAccessRecordMismatches(&planned, existingRecord)
			if mismatches != nil {
				resp.Diagnostics.AddError(
					"Cannot adopt existing access record",
					formatMismatches("authress_access_record", recordId, mismatches),
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

	tflog.Debug(ctx, "AccessRecord.Create succeeded", map[string]any{"recordId": recordId})
	diags = resp.State.Set(ctx, planned)
	resp.Diagnostics.Append(diags...)
}

func (r *AccessRecordInterfaceProvider) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var current AuthressAccessRecordResource
	diags := req.State.Get(ctx, &current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := current.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecord.Read starting", map[string]any{"recordId": recordId})

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

	current = mapSdkAccessRecordToTerraform(returnedRecord)
	diags = resp.State.Set(ctx, &current)
	resp.Diagnostics.Append(diags...)
}

func (r *AccessRecordInterfaceProvider) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var planned AuthressAccessRecordResource
	diags := req.Plan.Get(ctx, &planned)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := planned.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecord.Update starting", map[string]any{"recordId": recordId})

	sdkRecord := mapTerraformAccessRecordToSdk(&planned)

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

	tflog.Debug(ctx, "AccessRecord.Update succeeded", map[string]any{"recordId": recordId})
	diags = resp.State.Set(ctx, planned)
	resp.Diagnostics.Append(diags...)
}

func (r *AccessRecordInterfaceProvider) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var current AuthressAccessRecordResource
	diags := req.State.Get(ctx, &current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	recordId := current.RecordId.ValueString()
	tflog.Debug(ctx, "AccessRecord.Delete starting", map[string]any{"recordId": recordId})

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

func (r *AccessRecordInterfaceProvider) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("record_id"), req, resp)
}

// ─── SDK Mapping ─────────────────────────────────────────────────────────────

func mapTerraformAccessRecordToSdk(tf *AuthressAccessRecordResource) models.AccessRecord {
	record := models.AccessRecord{
		Name: tf.Name.ValueString(),
	}

	recordId := tf.RecordId.ValueString()
	record.SetRecordId(recordId)

	users := make([]models.User, 0, len(tf.Users))
	for _, u := range tf.Users {
		users = append(users, models.User{UserId: u.UserId.ValueString()})
	}
	record.SetUsers(users)

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

		statements = append(statements, models.Statement{
			Roles:     roles,
			Resources: resources,
		})
	}
	record.SetStatements(statements)

	return record
}

func mapSdkAccessRecordToTerraform(sdkRecord *models.AccessRecord) AuthressAccessRecordResource {
	tf := AuthressAccessRecordResource{
		RecordId: TerraformType.StringValue(sdkRecord.GetRecordId()),
		Name:     TerraformType.StringValue(sdkRecord.GetName()),
	}

	sdkUsers := sdkRecord.GetUsers()
	tf.Users = make([]AccessRecordUserResource, 0, len(sdkUsers))
	for _, u := range sdkUsers {
		tf.Users = append(tf.Users, AccessRecordUserResource{
			UserId: TerraformType.StringValue(u.UserId),
		})
	}

	sdkStatements := sdkRecord.GetStatements()
	tf.Statements = make([]AccessRecordStatementResource, 0, len(sdkStatements))
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

		tf.Statements = append(tf.Statements, AccessRecordStatementResource{
			Roles:     roles,
			Resources: resources,
		})
	}

	return tf
}

// ─── Adoption ────────────────────────────────────────────────────────────────

func collectAccessRecordMismatches(planned *AuthressAccessRecordResource, existing *models.AccessRecord) []FieldMismatch {
	checks := []*FieldMismatch{
		compareField("name", planned.Name.ValueString(), existing.GetName()),
	}

	// Compare users
	existingUsers := existing.GetUsers()
	if len(planned.Users) != len(existingUsers) {
		checks = append(checks, &FieldMismatch{
			Field:    "users (count)",
			Expected: fmt.Sprintf("%d", len(planned.Users)),
			Actual:   fmt.Sprintf("%d", len(existingUsers)),
		})
	} else {
		for i, u := range planned.Users {
			if i < len(existingUsers) {
				checks = append(checks, compareField(
					fmt.Sprintf("users[%d].user_id", i),
					u.UserId.ValueString(),
					existingUsers[i].UserId,
				))
			}
		}
	}

	// Compare statements
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

			// Compare roles
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
						fmt.Sprintf("statements[%d].roles[%d]", i, j),
						role,
						existingStmt.Roles[j],
					))
				}
			}

			// Compare resources
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
						res.ResourceUri.ValueString(),
						existingStmt.Resources[j].ResourceUri,
					))
				}
			}
		}
	}

	return collectMismatches(checks...)
}
