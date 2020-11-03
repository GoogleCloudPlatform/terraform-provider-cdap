<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
# cdap_gcs_artifact


# Example

```
locals {
    bucket = "aeba5c94-db31-451a-85ea-27047cbe133b"
}

resource "cdap_gcs_artifact" "gcs_whistler_1_0_0" {
  name             = "whistler-transform"
  version          = "1.0.0"
  json_config_path = "gs://${local.bucket}/packages/healthcare-mapping-transform/1.0.0/whistler-transform-1.0.0.json"
  jar_binary_path  = "gs://${local.bucket}/packages/healthcare-mapping-transform/1.0.0/whistler-transform-1.0.0.jar"
}
```

## Argument Reference

The following fields are supported:

* jar_binary_path
  (Required):
  The GCS path to the JAR binary for the artifact.

* json_config_path
  (Required):
  The GCS path to the JSON config of the artifact.

* name
  (Required):
  The name of the artifact.

* namespace
  (Optional):
  The name of the namespace in which this resource belongs. If not provided, the default namespace is used.

* version
  (Required):
  The version of the artifact. Must match the version in the JAR manifest.


