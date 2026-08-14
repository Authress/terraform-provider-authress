---
page_title: "authress_service_client Resource - Authress"
subcategory: ""
description: |-
  Manages an Authress Service Client. Service Clients are machine identities used for service-to-service authorization.
---

# authress_service_client (Resource)

Manages an Authress [Service Client](https://authress.io/knowledge-base/docs/category/service-clients). Service Clients are machine identities used for service-to-service authorization.

## Example Usage

### Basic service client with KMS-backed access key

```hcl
resource "authress_service_client" "lambda" {
  name = "My Lambda Service"

  options {
    grant_user_permissions_access = true
  }

  access_key {
    public_key = data.aws_kms_public_key.authress_service_client.public_key
  }
}
```

### Service client with inline access record (recommended for self-permissions)

When a service client needs permissions on resources, use inline `statement` blocks. This automatically upserts an access record with the same ID as the service client, granting it the specified roles.

```hcl
resource "authress_service_client" "lambda" {
  name = "Gitzi Lambda Service"

  options {
    grant_user_permissions_access = true
  }

  statement {
    roles = ["Authress:Owner"]

    resource {
      resource_uri = "*"
    }
  }

  statement {
    roles = ["ro_api_access"]

    resource {
      resource_uri = "apis/*"
    }
  }

  access_key {
    public_key = data.aws_kms_public_key.authress_service_client.public_key
  }
}
```

This is equivalent to creating a separate `authress_access_record` with `record_id = client_id` and `user { user_id = client_id }`, but more concise and guaranteed to stay in sync.

## Argument Reference

- `name` - (Required) The display name for the service client.
- `tags` - (Optional) Arbitrary key-value tags associated with the service client.

### `options` Block

- `grant_user_permissions_access` - (Optional, Default: `false`) Grant the client access to verify authorization on behalf of any user.
- `grant_token_generation` - (Optional, Default: `false`) Grant the client access to generate OAuth tokens on behalf of the Authress account.

### `access_key` Block

Repeatable. Each block registers a public key with the service client.

- `public_key` - (Required) The Ed25519 public key (base64 DER format).
- `key_id` - (Computed) The key ID assigned by Authress.

### `statement` Block

Optional, repeatable. Each block defines roles and resources to grant to the service client itself. Under the hood, this upserts an access record with `record_id = client_id`.

- `roles` - (Required) List of role IDs to grant (e.g. `Authress:Owner`).

#### `resource` Block (inside `statement`)

Repeatable. Each block specifies a resource the statement applies to.

- `resource_uri` - (Required) The resource URI pattern (e.g. `*` for all resources).

## Attributes Reference

- `client_id` - The unique identifier assigned by Authress on creation.
- `created_time` - The timestamp when the service client was created (RFC3339).

## Import

Service clients can be imported using their client ID:

```shell
terraform import authress_service_client.example sc_my-client-id
```
