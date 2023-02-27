package authress

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestRoleResource(t *testing.T) {
	resource.Test(t, resource.TestCase {
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: providerConfig + `
resource "authress_role" "test-100" {
	role_id = "test-1"
	name = "Terraform Test Role"
	permissions = {
		"one" = {
			"allow" = true
		}
	}
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("authress_role.test-100", "role_id", "test-1"),
					resource.TestCheckResourceAttr("authress_role.test-100", "permissions.one.allow", "true"),
					resource.TestCheckResourceAttrSet("authress_role.test-100", "last_updated"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "authress_role.test-100",
				ImportState:       true,
				ImportStateVerify: true,
				// The last_updated attribute does not exist in the Authress API, therefore there is no value for it during import.
				ImportStateVerifyIgnore: []string{"last_updated"},
			},
			// Update and Read testing
			{
				Config: providerConfig + `
resource "authress_role" "test-2" {
	role_id = "test-2"
	name = "Terraform Test Role 2"
	permissions = {
		"two" = {
			"allow" = true
		}
	}
}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("authress_role.test-2", "role_id", "test-2"),
					resource.TestCheckResourceAttr("authress_role.test-2", "permissions.two.allow", "true"),
					resource.TestCheckResourceAttrSet("authress_role.test-2", "last_updated"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}
