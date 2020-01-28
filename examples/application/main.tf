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

resource "cdap_application" "pipeline" {
    name = "example_pipeline"
    artifact {
        name = "cdap-data-pipeline"
        version = "6.1.1"
    }
    config = jsonencode({
        "resources": {
            "memoryMB": 2048,
            "virtualCores": 1
        },
        "driverResources": {
            "memoryMB": 2048,
            "virtualCores": 1
        },
        "connections": [
            {
                "from": "gcs_input",
                "to": "gcs_output"
            }
        ],
        "comments": [],
        "postActions": [],
        "properties": {},
        "processTimingEnabled": true,
        "stageLoggingEnabled": true,
        "stages": [
            {
                "name": "gcs_input",
                "plugin": {
                    "name": "GCSFile",
                    "type": "batchsource",
                    "label": "GCS Input",
                    "artifact": {
                        "name": "google-cloud",
                        "version": "0.13.2",
                        "scope": "SYSTEM"
                    },
                    "properties": {
                        "project": "auto-detect",
                        "format": "text",
                        "serviceFilePath": "auto-detect",
                        "filenameOnly": "false",
                        "recursive": "false",
                        "copyHeader": "false",
                        "schema": "{\"type\":\"record\",\"name\":\"etlSchemaBody\",\"fields\":[{\"name\":\"body\",\"type\":\"string\"}]}",
                        "path": "TODO",
                        "referenceName": "input"
                    }
                },
                "outputSchema": "{\"type\":\"record\",\"name\":\"etlSchemaBody\",\"fields\":[{\"name\":\"body\",\"type\":\"string\"}]}",
                "type": "batchsource",
                "label": "gcs_input",
                "icon": "fa-plug",
                "$$hashKey": "object:2909",
                "_uiPosition": {
                    "left": "880px",
                    "top": "550px"
                }
            },
            {
                "name": "gcs_output",
                "plugin": {
                    "name": "GCS",
                    "type": "batchsink",
                    "label": "GCS Output",
                    "artifact": {
                        "name": "google-cloud",
                        "version": "0.13.2",
                        "scope": "SYSTEM"
                    },
                    "properties": {
                        "project": "auto-detect",
                        "suffix": "yyyy-MM-dd-HH-mm",
                        "format": "json",
                        "serviceFilePath": "auto-detect",
                        "location": "us",
                        "schema": "{\"type\":\"record\",\"name\":\"etlSchemaBody\",\"fields\":[{\"name\":\"body\",\"type\":\"string\"}]}",
                        "referenceName": "gcs_output",
                        "path": "TODO"
                    }
                },
                "outputSchema": "{\"type\":\"record\",\"name\":\"etlSchemaBody\",\"fields\":[{\"name\":\"body\",\"type\":\"string\"}]}",
                "inputSchema": [
                    {
                        "name": "gcs_input",
                        "schema": "{\"type\":\"record\",\"name\":\"etlSchemaBody\",\"fields\":[{\"name\":\"body\",\"type\":\"string\"}]}"
                    }
                ],
                "type": "batchsink",
                "label": "gcs_output",
                "icon": "fa-plug",
                "$$hashKey": "object:2911",
                "_uiPosition": {
                    "left": "1180px",
                    "top": "550px"
                }
            }
        ],
        "schedule": "0 * * * *",
        "engine": "spark",
        "numOfRecordsPreview": 100,
        "maxConcurrentRuns": 1
    })
}
