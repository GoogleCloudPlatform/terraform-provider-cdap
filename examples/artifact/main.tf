// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
locals {
  hub_bucket = "aeba5c94-db31-451a-85ea-27047cbe133b"
}

resource "google_data_fusion_instance" "instance" {
  provider = google-beta
  name = "example"
  region = "us-central1"
  type = "BASIC"
  project = "example-project"

  # Use Healthcare Hub.
  options = {
    "market.base.url": "https://storage.googleapis.com/${local.hub_bucket}"
  }
}

data "google_client_config" "current" {}

provider "cdap" {
  host = "${google_data_fusion_instance.instance.service_endpoint}/api/"
  token = data.google_client_config.current.access_token
}

# Option 1: Path in GCS bucket containing the spec, JAR and JSON config.
resource "cdap_gcs_artifact" "gcs_whistler_1_0_0" {
  name = "whistler-transform"
  version = "1.0.0"
  json_config_path = "gs://${local.hub_bucket}/packages/healthcare-mapping-transform/1.0.0/whistler-transform-1.0.0.json"
  jar_binary_path = "gs://${local.hub_bucket}/packages/healthcare-mapping-transform/1.0.0/whistler-transform-1.0.0.jar"
}

# Option 2: Download or compile JAR and JSON config and pass as local files.
resource "cdap_local_artifact" "local_whistler_1_0_0" {
  name = "whistler-transform"
  version = "1.0.0"
  json_config_path = "./TODO/whistler-transform-1.0.0.json"
  jar_binary_path = "./TODO/whistler-transform-1.0.0.jar"
}
