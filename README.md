# Terraform Provider Cloudflare Ext

A minimal Terraform provider for Cloudflare resources that require write-only attribute support (Terraform 1.11+). Prevents secrets from being stored in Terraform state.

## Documentation

Full, comprehensive documentation is available on the [Terraform Registry](https://registry.terraform.io/providers/kenchan0130/cloudflareext). [API documentation](https://developers.cloudflare.com/api/) is also available for non-Terraform or service specific information.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.11
- [Go](https://golang.org/doc/install)

## Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the `make` command:

```shell
make build
```

## Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

## Using the provider

```terraform
terraform {
  required_providers {
    cloudflareext = {
      source  = "kenchan0130/cloudflareext"
      version = "~> 0.1.0"
    }
  }
}

provider "cloudflareext" {
  api_token  = var.cloudflare_api_token
  account_id = var.cloudflare_account_id
}
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `make build`. This will build the provider and put the provider binary in the current directory.

To generate or update documentation, run `make generate`.

To run the full suite of unit tests, run `make test`.

To run acceptance tests, run `make testacc`.

*Note:* Acceptance tests create real resources and require `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID` environment variables.

## Release

We use release management by [tagpr](https://github.com/Songmu/tagpr). When merging tagpr PR, next version would be released by github-actions.

## Contribution

See also [CONTRIBUTING.md](CONTRIBUTING.md).

### DCO Sign-Off Methods

The sign-off is a simple line at the end of the explanation for the patch, which certifies that you wrote it or otherwise have the right to pass it on as an open-source patch.

The DCO requires a sign-off message in the following format appear on each commit in the pull request:

```txt
Signed-off-by: Sample Developer sample@example.com
```

The text can either be manually added to your commit body, or you can add either `-s` or `--signoff` to your usual `git` commit commands.

#### Auto sign-off

The following method is examples only and are not mandatory.

```sh
touch .git/hooks/prepare-commit-msg
chmod +x .git/hooks/prepare-commit-msg
```

Edit the `prepare-commit-msg` file like:

```sh
#!/bin/sh

name=$(git config user.name)
email=$(git config user.email)

if [ -z "${name}" ]; then
  echo "empty git config user.name"
  exit 1
fi

if [ -z "${email}" ]; then
  echo "empty git config user.email"
  exit 1
fi

git interpret-trailers --if-exists doNothing --trailer \
    "Signed-off-by: ${name} <${email}>" \
    --in-place "$1"
```
