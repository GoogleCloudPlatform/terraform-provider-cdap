<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
The CDAP provider is used to configure your
[CDAP](https://docs.cdap.io/cdap/current/en/index.html) infrastructure.

## Installation

-  Download the provider binary from the
   [releases page](https://github.com/GoogleCloudPlatform/terraform-provider-cdap/releases)
   page.

-  Rename the binary to match the
  [pattern](https://www.terraform.io/docs/configuration/providers.html#plugin-names-and-versions)
  `terraform-provider-cdap_vX.Y.Z`).

-  Run `chmod u+x ./terraform-provider-cdap_vX.Y.Z` to make the binary
   executable.

-  Move the binary to a location your Terraform configs can
   [find it](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins).

## Usage

An example of the CDAP provider initialized on a GCP Cloud Data Fusion instance:

```
resource "google_data_fusion_instance" "instance" {
  provider = google-beta
  name     = "example"
  region   = "us-central1"
  type     = "BASIC"
  project  = "example-project"
}

data "google_client_config" "current" {}

provider "cdap" {
  host  = "${google_data_fusion_instance.instance.service_endpoint}/api/"
  token = data.google_client_config.current.access_token
}
```

## Argument Reference

The following fields are supported:

* host
  (Required):
  The address of the CDAP instance.

* token
  (Optional):
  The OAuth token to use for all http calls to the instance.



## Resources

* [cdap_application](r/cdap_application.md)
* [cdap_gcs_artifact](r/cdap_gcs_artifact.md)
* [cdap_local_artifact](r/cdap_local_artifact.md)
* [cdap_namespace](r/cdap_namespace.md)
* [cdap_namespace_preferences](r/cdap_namespace_preferences.md)
* [cdap_profile](r/cdap_profile.md)
* [cdap_streaming_program_run](r/cdap_streaming_program_run.md)
