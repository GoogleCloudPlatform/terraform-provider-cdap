resource "google_data_fusion_instance" "instance" {
  provider = google-beta
  name = "example"
  region = "us-central1"
  type = "BASIC"
  project = "example-project"
}

data "google_client_config" "current" {}

provider "cdap" {
  host = "${google_data_fusion_instance.instance.service_endpoint}/api/"
  token = data.google_client_config.current.access_token
}

resource "cdap_application" "app" {
  name = "example-app"
  config = templatefile("${path.module}/pipeline.json", {
    input_bucket = "gs://example-bucket",
    output_bucket = "gs://output-bucket",
  })
}
