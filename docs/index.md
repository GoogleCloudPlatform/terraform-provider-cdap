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

## Retries

Retries are off by default. Opt in with a `retry` block:

```
provider "cdap" {
  host  = "${google_data_fusion_instance.instance.service_endpoint}/api/"
  token = data.google_client_config.current.access_token

  retry {
    enabled         = true # default is false; set to true to enable retries
    timeout_seconds = 90   # default is 90s when enabled is set to true; no new attempt starts after this (one in-flight request may still run up to the 30m client timeout)
    max_attempts    = 3    # default is 3 when enabled is set to true; 0 = limited only by timeout_seconds
  }
}
```

* Connection errors and transient HTTP status codes (`408`, `429`, `500`, `502`, `503`, `504`) are retried with exponential backoff (0.5s, 1s, 2s, … capped at 10s) until `timeout_seconds` elapses or `max_attempts` is reached, whichever comes first. For large (>100 MB) artifact JARs where a single timed-out attempt can take 60–120s, increase `timeout_seconds` (for example to `300`) so the retry window accommodates multiple attempts.
* Non-idempotent calls are never retried: `cdap_streaming_program_run` start and `cdap_oauth_credential` creation.
* Artifact uploads are reconciled for versions that were absent before the upload: a `409` returned after a retry, or a failed final attempt after which the version exists, is treated as successful. A version that already existed is never adopted.

Retry decisions are logged at `TF_LOG=WARN` and above.

## Argument Reference

The following fields are supported:

* host
  (Required):
  The address of the CDAP instance.

* retry
  (Optional):
  Retry policy for transient API failures. When omitted, every API call is attempted exactly once.

* retry.enabled
  (Optional):
  Whether retries are active. Defaults to false.

* retry.max_attempts
  (Optional):
  Maximum number of attempts per API call, including the first. Defaults to 3 when enabled is set to true; 0 means attempts are limited only by timeout_seconds. Whichever limit is reached first stops retrying.

* retry.timeout_seconds
  (Optional):
  No new attempt is started once this many seconds have elapsed since the first attempt began. A single in-flight request is bounded by the HTTP client timeout, not this value. Defaults to 90 (90s) when enabled is set to true.

* token
  (Optional):
  The OAuth token to use for all http calls to the instance.


