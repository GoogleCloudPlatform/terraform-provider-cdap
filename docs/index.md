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

## Retrying transient errors

By default every CDAP API call is attempted exactly once. Opt in to automatic
retries with the `retry` block:

```
provider "cdap" {
  host  = "${google_data_fusion_instance.instance.service_endpoint}/api/"
  token = data.google_client_config.current.access_token

  retry {
    # enabled     = true                      # default when the block is present
    # timeout     = 90                        # seconds, measured from the first failure
    # error_codes = [429, 500, 502, 503, 504] # default allowlist
  }
}
```

Behaviour when retries are enabled:

* Connection-level failures (resets, EOFs, timeouts) and responses whose status
  code is in `error_codes` are retried with exponential backoff and jitter
  (1s → 30s), honouring a `Retry-After` header when the server sends one.
* Retries stop once `timeout` seconds have elapsed since the first failure. The
  final error reports the number of attempts and the last response received.
* `501 Not Implemented`, cancelled operations and TLS/certificate errors are
  never retried, regardless of configuration.
* Requests whose replay could create duplicate side effects are never retried:
  starting a `cdap_streaming_program_run` and exchanging the one-time code of a
  `cdap_oauth_credential`.
* If an artifact upload or a profile disable returns `409 Conflict` **after** a
  retry, the provider verifies the intended state (artifact version present /
  profile disabled) and treats it as success. A `409` on the first attempt is
  still reported as an error.

If the instance sits behind an authenticating reverse proxy that can
intermittently answer `401`/`403` during its own backend hiccups, those codes
can be added explicitly. Note that an invalid token will then take `timeout`
seconds to fail instead of failing immediately:

```
provider "cdap" {
  host  = "${google_data_fusion_instance.instance.service_endpoint}/api/"
  token = data.google_client_config.current.access_token

  retry {
    timeout     = 120
    error_codes = [401, 403, 429, 500, 502, 503, 504]
  }
}
```

Set `TF_LOG=INFO` (or `DEBUG`) to see each retry decision in the Terraform log.

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
  Whether retries are active. Defaults to true when the retry block is present.

* retry.error_codes
  (Optional):
  HTTP status codes treated as transient and retried. Connection errors are always retried. Defaults to [429, 500, 502, 503, 504].

* retry.timeout
  (Optional):
  Maximum time in seconds, measured from the first failure, during which a failed API call is retried. Defaults to 90.

* token
  (Optional):
  The OAuth token to use for all http calls to the instance.


