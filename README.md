# Terraform CDAP Provider

This
[custom provider](https://www.terraform.io/docs/extend/writing-custom-providers.html)
for Terraform can be used to manage a CDAP API (exposed for example by a
[Cloud Data Fusion](https://cloud.google.com/data-fusion/) Instance) in an
infra-as-code manner.

This is a
[community maintained provider](https://www.terraform.io/docs/providers/type/community-index.html)
and is not officially tested or maintained by Hashicorp or Google.

## Installation

-   Build the provider by running `go build` from the root directory (see
    [here](https://golang.org/cmd/go/#hdr-Compile_packages_and_dependencies)).

-   Move the binary to a location your Terraform configs can find it (see
    [here](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins)).

## Usage

See the *examples/* directory.

## Bugs & Feature Requests

Please file issues in Github.
