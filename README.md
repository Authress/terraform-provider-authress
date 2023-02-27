# Authress Terraform Provider
The Authress terraform provider to automatically configure Authress from Terraform

[![GitHub Workflow][workflow]][workflow-link] [![Forums][discuss-badge]][discuss] [![Terraform][terraform-badge]][terraform-link]

[workflow]: https://github.com/authress/terraform-provider-authress/actions/workflows/build.yml/badge.svg
[workflow-link]: https://github.com/Authress/terraform-provider-authress/actions

[discuss-badge]: https://img.shields.io/badge/build-terraform--authress-623CE4.svg
[discuss]: https://discuss.hashicorp.com/c/terraform-providers/31

[terraform-badge]: https://img.shields.io/badge/install-terraform--authress-blue.svg
[terraform-link]: https://registry.terraform.io/providers/hashicorp/authress/latest/docs

## Installation

Install the `Authress` terraform provider, and review the documentation @ [Authress Terraform Documentation](https://registry.terraform.io/providers/hashicorp/authress/latest/docs)

```hcl
terraform {
  required_providers {
    authress = {
      source  = "authress/authress"
      
      # Specify your Authress custom domain, configured at https://authress.io/app/#/settings?focus=domain
      custom_domain = "https://login.example.com"
    }
  }
}
```


## Development
For developing this plugin see more information in [Development Docs](./development-examples/README.md).