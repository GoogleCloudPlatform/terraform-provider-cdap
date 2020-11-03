# Terraform CDAP Provider

This
[custom provider](https://www.terraform.io/docs/extend/writing-custom-providers.html)
for Terraform can be used to manage a
[CDAP](https://docs.cdap.io/cdap/current/en/index.html) API (exposed for example by a
[GCP Cloud Data Fusion](https://cloud.google.com/data-fusion/) Instance) in an
infra-as-code manner.

This is a
[community maintained provider](https://www.terraform.io/docs/providers/type/community-index.html)
and not an official Google or Hashicorp product.

GCP Data Fusion specific helpers and modules can be found in the corresponding
[Cloud Foundation Toolkit repo](https://github.com/terraform-google-modules/terraform-google-data-fusion).

## Documentation

- Website: https://registry.terraform.io/providers/GoogleCloudPlatform/cdap/
- Blog post: https://cloud.google.com/blog/products/data-analytics/open-source-etl-pipeline-tool

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md)

## Development

To build a local version of the provider, run `go build -o ${test_dir}` 
where `test_dir` is the path to a directory hosting test Terraform configs.

## Releasing

Automated releases are handled by Github Actions.

1. Choose a version. It should match the regex `^v[0-9]+\.[0-9]+\.[0-9]+$`.
   That is, a leading "v", followed by three period-separated numbers.

   ```bash
   version="v0.1.0"
   ```

1. Create the Git tag.

   For binaries:

   ```bash
   git tag -a "${version}" -m "${version}"
   ```

1. Push the tag:

   ```bash
   git push origin --tags
   ```
