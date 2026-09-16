<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
## Usage

An example of the CDAP provider initialized on a GCP Cloud Data Fusion instance:

```
terraform {
  required_providers {
    cdap = {
      source = "GoogleCloudPlatform/cdap"
      # Pin to a specific version as 0.x releases are not guaranteed to be backwards compatible.
      version = "0.9.0"
    }
  }
}

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

* retry_timeout
  (Optional):
  The maximum duration in seconds to retry an API call that fails with a transient error (any connection failure or HTTP 4xx/5xx except 401 and 404). Defaults to 90.

* token
  (Optional):
  The OAuth token to use for all http calls to the instance.


