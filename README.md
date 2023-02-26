# Authress Terraform Provider
The Authress terraform provider to automatically configure Authress from Terraform

![example workflow](https://github.com/authress/terraform-provider-authress/actions/workflows/build.yml/badge.svg) [![Forums][discuss-badge]][discuss]

[discuss-badge]: https://img.shields.io/badge/discuss-terraform--authress-623CE4.svg?style=flat
[discuss]: https://discuss.hashicorp.com/c/terraform-providers/31

## Development

### Setup

Update your `~/.terraformrc` with the following
```hcl
provider_installation {

  dev_overrides {
      "localhost.com/authress/authress" = "~/go/bin"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

Test the provider out locally to validate it works
```sh
go run main.go
```

Run the following command to build the provider

```shell
go build -o terraform-provider-authress
```

#### Update Dependencies
Run:

```shell
go mod tidy
```

## Test sample configuration

First, build and install the provider.

```shell
make install
```

Then, run the following command to initialize the workspace and apply the sample configuration.

```shell
terraform init && terraform apply
```