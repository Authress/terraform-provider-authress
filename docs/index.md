---
page_title: "Authress Provider"
description: |-
  Used to interact with your Authress roles, resources, permissions and APIs.
---

# Authress Provider

Deploys resources to your [Authress](https://authress.io) account using Terraform.

Authress is a User Authorization API for software makers, and provides granular, role and resource-based access for the users of your software.

The [Authress Knowledge Base](https://authress.io/knowledge-base/docs/category/introduction) contains examples and recommendations on configuring your Authress account.

### Example Usage

```hcl
terraform {
  required_providers {
    authress = {
      source  = "authress/authress"
      version = "~> 2.0"
    }
  }
}

provider "authress" {
  # See: https://authress.io/knowledge-base/docs/category/cicd
}

resource "authress_role" "document_admin" {
  role_id = "ro_documents_admin"
  name = "Documents Administrator"
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

## Authentication

The provider authenticates using a JWT token. The recommended approach is to configure CI/CD OIDC integration so your pipeline automatically provides a token via the `AUTHRESS_KEY` environment variable:

- [GitHub Actions OIDC](https://authress.io/knowledge-base/docs/cicd/github)
- [GitLab Pipelines OIDC](https://authress.io/knowledge-base/docs/cicd/gitlab)
- [Quick start guide](https://authress.io/app/#/settings?focus=quick&flow=oidc)

The provider extracts your Authress account ID and custom domain automatically from the token's `aud` claim — no additional configuration is needed.

## Argument Reference

- `access_key` `string` (Optional, Sensitive) - A JWT token for the Authress API. Defaults to the `AUTHRESS_KEY` environment variable. Prefer setting via CI/CD OIDC rather than hardcoding.

## Source Code on GitHub
The Source for this provider is available in the [Authress Terraform Provider GitHub](https://github.com/Authress/terraform-provider-authress) repository.