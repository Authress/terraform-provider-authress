terraform {
  required_providers {
    authress = {
      version = "0.2"
      source  = "localhost.com/authress/authress"
    }
  }
}

provider "authress" {}

# variable "coffee_name" {
#   type    = string
#   default = "Vagrante espresso"
# }

# data "authress_coffees" "all" {}

# # Returns all coffees
# output "all_coffees" {
#   value = data.authress_coffees.all.coffees
# }

# # Only returns packer spiced latte
# output "coffee" {
#   value = {
#     for coffee in data.authress_coffees.all.coffees :
#     coffee.id => coffee
#     if coffee.name == var.coffee_name
#   }
# }

# output "psl" {
#   value = module.psl.coffee
# }
