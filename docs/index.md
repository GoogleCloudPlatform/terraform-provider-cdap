<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
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
