terraform {
  required_providers {
    authress = {
      version = "0.2"
      source  = "localhost.com/authress/authress"
    }
  }
}

provider "authress" {}
