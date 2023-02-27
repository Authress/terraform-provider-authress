terraform {
  required_providers {
    authress = {
      version = "0.2"
      source  = "hashicorp.com/authress/authress"
    }
  }
}

provider "authress" {}
