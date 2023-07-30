terraform {
  required_providers {
    authress = {
      version = "~> 2.0.35"
      source  = "authress/authress"
    }
  }
}

provider "authress" {
  custom_domain = "authress-test.authress.com"
}

# resource "authress_role" "test-100" {
#     role_id = "ro_test-1"
#     name = "Terraform Test Role"
#     permissions = {
#       "one" = {
#         "allow" = true
#       }

#       "two" = {
#         "allow" = true
#         "grant" = true
#       }
#     }
#   }