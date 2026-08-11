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
    public_key = "MCowBQYDK2VwAyEAexamplekey1base64encoded"
  }

  access_key {
    public_key = "MCowBQYDK2VwAyEAexamplekey2base64encoded"
  }
}
