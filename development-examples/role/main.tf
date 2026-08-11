terraform {
  required_providers {
    authress = {
      source = "authress/authress"
    }
  }
}

provider "authress" {
  # Uses AUTHRESS_KEY environment variable automatically
}

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
