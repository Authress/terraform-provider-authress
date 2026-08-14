---
page_title: "authress_access_record Resource - Authress"
subcategory: ""
description: |-
  Manages an Authress Access Record. Grants users permissions on resources via role assignments.
---

# authress_access_record (Resource)

Manages an Authress [Access Record](https://authress.io/knowledge-base/docs/category/access-records). Grants users permissions on resources via role assignments.

Users are defined at the record level and apply to all statements. For per-statement user/group targeting, use [`authress_access_record_granular`](access_record_granular.md).

## Example Usage

### Grant a service client full owner access

```hcl
resource "authress_access_record" "service_client_owner" {
  record_id = authress_service_client.lambda.client_id
  name      = "Service Client Owner Access"

  user {
    user_id = authress_service_client.lambda.client_id
  }

  statement {
    roles = ["Authress:Owner"]

    resource {
      resource_uri = "*"
    }
  }
}
```

### Grant a user access to multiple resource paths

```hcl
resource "authress_access_record" "editor_access" {
  record_id = "rec_editor-access"
  name      = "Editor Access"

  user {
    user_id = "user_001"
  }
  user {
    user_id = "user_002"
  }

  statement {
    roles = ["ro_editor"]

    resource {
      resource_uri = "projects/proj_abc"
    }
    resource {
      resource_uri = "projects/proj_def"
    }
  }

  statement {
    roles = ["ro_viewer"]

    resource {
      resource_uri = "reports/*"
    }
  }
}
```

## Argument Reference

- `record_id` - (Required, Forces new resource) The unique identifier for the access record.
- `name` - (Required) A helpful name for this access record.

### `user` Block

Repeatable. Each block adds a user or service client to the record.

- `user_id` - (Required) The user ID or service client ID.

### `statement` Block

Repeatable. Each block defines a set of roles granted on a set of resources.

- `roles` - (Required) List of role IDs to grant (e.g. `Authress:Owner`, `ro_editor`).

#### `resource` Block (inside `statement`)

Repeatable. Each block specifies a resource the statement applies to.

- `resource_uri` - (Required) The resource URI pattern (e.g. `*` for all resources, `projects/proj_abc` for a specific resource).

## Import

Access records can be imported using their record ID:

```shell
terraform import authress_access_record.example rec_my-record-id
```
