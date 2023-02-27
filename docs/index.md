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
required_providers {
  authress = {
    source  = "authress/authress"
    version = "~> 1.0"
  }
}

provider "authress" {
  # Authress custom domain configuration: https://authress.io/app/#/settings?focus=domain
  custom_domain = "https://login.example.com"
}

resource "authress_role" "document_admin" {
  role_id = "documents_admin"
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

## Argument Reference

- `access_key` - `(string)` - The access key for the Authress API. Should be [configured by your CI/CD](https://authress.io/knowledge-base/docs/category/cicd) for more information. Or it can be overridden directly here. Do not commit this plaintext value to your source code.
- `custom_domain` - `(string)` - Your Authress custom domain. [Configured a custom domain for Account](https://authress.io/app/#/settings?focus=domain) or use [provided domain](https://authress.io/app/#/api?route=overview).

## Source Code on GitHub
The Source for this provider is available in the [Authress Terraform Provider GitHub](https://github.com/Authress/terraform-provider-authress) repository.