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

# https://github.com/data-integrations/trash
resource "cdap_artifact" "trash" {
  name = "trash-plugin"
  version = "1.2.0"
  extends = [
    "system:cdap-data-pipeline[6.0.0-SNAPSHOT,7.0.0-SNAPSHOT)",
    "system:cdap-data-streams[6.0.0-SNAPSHOT,7.0.0-SNAPSHOT)",
    "system:cdap-etl-batch[6.0.0-SNAPSHOT,7.0.0-SNAPSHOT)",
    "system:cdap-etl-realtime[6.0.0-SNAPSHOT,7.0.0-SNAPSHOT)",
  ]
  jar_binary_path = "${path.module}/trash-plugin-1.2.0.jar"
}

resource "cdap_artifact_property" "trash_batchsink" {
  name = "widgets.Trash-batchsink"
  artifact_name = cdap_artifact.trash.name
  artifact_version = cdap_artifact.trash.version
  value = "{\"metadata\":{\"spec-version\":\"1.0\"},\"configuration-groups\":[{\"label\":\"Trash Configuration\",\"properties\":[{\"widget-type\":\"textbox\",\"label\":\"Reference Name\",\"name\":\"referenceName\",\"description\":\"Reference specifies the name to be used to track this external source\",\"widget-attributes\":{\"default\":\"Trash\"}}]}]}"
}

resource "cdap_artifact_property" "trash_doc" {
  name = "widgets.Trash-batchsink"
  artifact_name = cdap_artifact.trash.name
  artifact_version = cdap_artifact.trash.version
  value = "# Trash\n\nTrash consumes all the records on the input and eats them all,\nmeans no output is generated or no output is stored anywhere.\n"
}
