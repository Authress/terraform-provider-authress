<p id="main" align="center">
  <img src="https://authress.io/static/images/linkedin-banner.png" alt="Authress media banner">
</p>

# Authress Terraform Provider

Manage [Authress](https://authress.io) resources declaratively from Terraform or OpenTofu.

[![GitHub Workflow][workflow]][workflow-link] [![Forums][discuss-badge]][discuss] [![Terraform][terraform-badge]][terraform-link]

[workflow]: https://github.com/authress/terraform-provider-authress/actions/workflows/build.yml/badge.svg
[workflow-link]: https://github.com/Authress/terraform-provider-authress/actions
[discuss-badge]: https://img.shields.io/badge/build-terraform--authress-623CE4.svg
[discuss]: https://discuss.hashicorp.com/c/terraform-providers/31
[terraform-badge]: https://img.shields.io/badge/install-terraform--authress-blue.svg
[terraform-link]: https://registry.terraform.io/providers/authress/authress/latest/docs

## Installation

Install the `Authress` terraform provider and review the full documentation at the [Terraform Registry](https://registry.terraform.io/providers/authress/authress/latest/docs).

```hcl
terraform {
  required_providers {
    authress = {
      source = "authress/authress"
    }
  }
}

provider "authress" {}
```

Authentication is configured automatically via your CI/CD pipeline's OIDC integration. See the [Authress CI/CD setup guide](https://authress.io/knowledge-base/docs/category/cicd) for GitHub Actions, GitLab CI, and other platforms.

## Resources

### `authress_role`

Manages a role containing a set of permissions.

```hcl
resource "authress_role" "editor" {
  role_id     = "ro_editor"
  name        = "Editor"
  description = "Can read and write content"

  permissions = {
    "documents:read" = {
      allow = true
    }
    "documents:write" = {
      allow    = true
      grant    = true
      delegate = false
    }
  }
}
```

### `authress_service_client`

Manages a service client (machine identity) with optional access keys.

```hcl
resource "authress_service_client" "api_worker" {
  client_id = "sc_api_worker"
  name      = "API Worker"

  tags = {
    environment = "production"
    team        = "backend"
  }

  options {
    grant_user_permissions_access = true
  }

  access_key {
    public_key = "MCowBQYDK2VwAyEA..."
  }
}
```

## Development

For developing this plugin see [Development Docs](./development-examples/README.md).
