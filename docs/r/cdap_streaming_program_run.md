<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
# cdap_streaming_program_run


# Example

```
resource "cdap_streaming_program_run" "test" {
  namespace = "adp_staging"
  app       = "HL7v2_to_fhir"

  runtime_arguments = {
    "system.profile.name" = "my-custom-profile-name"
  }
}
```

## Argument Reference

The following fields are supported:

* app
  (Required):
  Name of the application.

* namespace
  (Optional):
  The name of the namespace in which this resource belongs. If not provided, the default namespace is used.

* program
  (Required):
  Name of the program.

* run_id
  (Computed):
  The run the CDAP Run ID

* runtime_arguments
  (Required):
  The runtime arguments used to start the program

* type
  (Required):
  One of flows, mapreduce, services, spark, workers, or workflows.


