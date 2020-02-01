<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
# CDAP Provider


## Installation

-  Download the provider binary from the
   [releases page](https://github.com/GoogleCloudPlatform/terraform-provider-cdap/releases)
   page.

-  Move the binary to a location your Terraform configs can
   [find it]](https://www.terraform.io/docs/configuration/providers.html#third-party-plugins).

## Usage

In your Terraform config, add a provider block as follows:

```
provider "cdap" {
    host  = "<HOST>"
    token = "<TOKEN>"
}
```

An example of a CDAP provider initialized on GCP Cloud Data Fusion:

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
  (Required):
  The OAuth token to use for all http calls to the instance.



## Resources

* [cdap_application](r/cdap_application)
* [cdap_gcs_artifact](r/cdap_gcs_artifact)
* [cdap_local_artifact](r/cdap_local_artifact)
* [cdap_namespace](r/cdap_namespace)
* [cdap_namespace_preferences](r/cdap_namespace_preferences)
* [cdap_profile](r/cdap_profile)
