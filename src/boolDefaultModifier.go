package authress

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	TerraformType "github.com/hashicorp/terraform-plugin-framework/types"
)

// stringDefaultModifier is a plan modifier that sets a default value for a
// types.boolType attribute when it is not configured. The attribute must be
// marked as Optional and Computed. When setting the state during the resource
// Create, Read, or Update methods, this default value must also be included or
// the Terraform CLI will generate an error.
type BoolDefaultModifier struct {
    Default bool
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m BoolDefaultModifier) Description(ctx context.Context) string {
    return fmt.Sprintf("If value is not configured, defaults to %t", m.Default)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (m BoolDefaultModifier) MarkdownDescription(ctx context.Context) string {
    return fmt.Sprintf("If value is not configured, defaults to `%t`", m.Default)
}

// PlanModifybool runs the logic of the plan modifier.
// Access to the configuration, plan, and state is available in `req`, while
// `resp` contains fields for updating the planned value, triggering resource
// replacement, and returning diagnostics.
func (m BoolDefaultModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
    if !req.ConfigValue.IsNull() {
        return
    }

    // If the attribute plan is "known" and "not null", then a previous plan modifier in the sequence
	// has already been applied, and we don't want to interfere.
	if !req.PlanValue.IsUnknown() && !req.PlanValue.IsNull() {
		return
	}

    resp.PlanValue = TerraformType.BoolValue(m.Default)
}

func boolDefault(defaultValue bool) planmodifier.Bool {
    return BoolDefaultModifier {
        Default: defaultValue,
    }
}