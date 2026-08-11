# Authress Terraform Provider

Manage [Authress](https://authress.io) resources — roles, service clients, and access policies — directly from Terraform.

[![GitHub Workflow][workflow]][workflow-link] [![Terraform][terraform-badge]][terraform-link]

[workflow]: https://github.com/authress/terraform-provider-authress/actions/workflows/build.yml/badge.svg
[workflow-link]: https://github.com/Authress/terraform-provider-authress/actions
[terraform-badge]: https://img.shields.io/badge/install-terraform--authress-blue.svg
[terraform-link]: https://registry.terraform.io/providers/authress/authress/latest/docs

## Authentication

The provider uses zero-config authentication via the `AUTHRESS_KEY` environment variable. Set it to an OIDC JWT issued for your Authress account:

```sh
export AUTHRESS_KEY="eyJhbGciOi..."
```

No provider-block configuration is required when the environment variable is set.

## Provider Configuration

```hcl
terraform {
  required_providers {
    authress = {
      source = "authress/authress"
    }
  }
}

provider "authress" {
  # Optional: override the environment variable
  # access_key = "eyJhbGciOi..."
}
```

## Resources

### `authress_role`

Manages an Authress role with permission definitions.

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
      allow = true
    }
  }
}
```

### `authress_service_client`

Manages an Authress service client with access keys, tags, and options.

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
    grant_token_generation        = false
  }

  access_key {
    public_key = "MCowBQYDK2VwAyEA..."
  }
}
```

## Go SDK

This provider depends on [authress-sdk.go](https://github.com/Authress/authress-sdk.go) for API communication. The SDK is referenced via a `replace` directive in `go.mod` for local development.

## Examples

See [`development-examples/`](./development-examples/) for working configurations:

- [`role/`](./development-examples/role/) — Role with permissions
- [`service_client/`](./development-examples/service_client/) — Service client with keys and tags

## Development

For developing this plugin see [Development Docs](./development-examples/README.md).

### Generating Documentation

```sh
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate
```
