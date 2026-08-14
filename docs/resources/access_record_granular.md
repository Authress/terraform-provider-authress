---
page_title: "authress_access_record_granular Resource - Authress"
subcategory: ""
description: |-
  Manages an Authress Access Record with per-statement user and group assignments.
---

# authress_access_record_granular (Resource)

Manages an Authress [Access Record](https://authress.io/knowledge-base/docs/category/access-records) with per-statement user and group assignments. Use when different statements need to target different principals.

For simpler records where all users share the same statements, use [`authress_access_record`](access_record.md).

## Example Usage

### Different roles for different users in a single record

```hcl
resource "authress_access_record_granular" "project_access" {
  record_id = "rec_project-abc"
  name      = "Project ABC Access"

  statement {
    roles = ["ro_admin"]

    resource {
      resource_uri = "projects/proj_abc"
    }

    user {
      user_id = "user_admin"
    }
  }

  statement {
    roles = ["ro_viewer"]

    resource {
      resource_uri = "projects/proj_abc"
    }
    resource {
      resource_uri = "projects/proj_abc/reports/*"
    }

    user {
      user_id = "user_viewer_1"
    }
    user {
      user_id = "user_viewer_2"
    }
    group {
      group_id = "grp_engineering"
    }
  }
}
```

### Group-based access

```hcl
resource "authress_access_record_granular" "team_access" {
  record_id = "rec_team-access"
  name      = "Team Access"

  statement {
    roles = ["ro_editor"]

    resource {
      resource_uri = "documents/*"
    }

    group {
      group_id = "grp_content-team"
    }
  }
}
```

## Argument Reference

- `record_id` - (Required, Forces new resource) The unique identifier for the access record.
- `name` - (Required) A helpful name for this access record.

### `statement` Block

Repeatable. Each block defines roles, resources, and the principals (users/groups) they apply to.

- `roles` - (Required) List of role IDs to grant (e.g. `Authress:Owner`, `ro_editor`).

#### `resource` Block (inside `statement`)

Repeatable. Each block specifies a resource the statement applies to.

- `resource_uri` - (Required) The resource URI pattern (e.g. `*` for all resources).

#### `user` Block (inside `statement`)

Repeatable. Each block adds a user or service client to this specific statement.

- `user_id` - (Required) The user ID or service client ID.

#### `group` Block (inside `statement`)

Repeatable. Each block adds a group to this specific statement.

- `group_id` - (Required) The group ID.

## Import

Access records can be imported using their record ID:

```shell
terraform import authress_access_record_granular.example rec_my-record-id
```
