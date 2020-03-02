<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
# cdap_streaming_program


# Example

```
resource "cdap_streaming_program" "test" {
  namespace = "staging"
  type      = "spark"
  app       = "HL7v2_to_fhir"

  runtime_arguments = {
    "system.profile.name" = "hl7-stream-ingest"
  }
}
```

## Argument Reference

The following fields are supported:

* app
  (Required):
  Name of the application.

* name
  (Required):
  Name of the program.

* namespace
  (Optional):
  The name of the namespace in which this resource belongs. If not provided, the default namespace is used.

* runtime_arguments
  (Required):
  The runtime arguments used to start the program

* type
  (Required):
  One of flows, mapreduce, services, spark, workers, or workflows.


