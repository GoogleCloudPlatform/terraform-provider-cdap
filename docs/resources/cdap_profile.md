<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
# cdap_profile


# Example

```
resource "cdap_profile" "profile" {
    name  = "example-profile"
    label = "example-profile"
    profile_provisioner {
        name = "gcp-dataproc"
        properties {
            name        = "projectId"
            value       = "example-project"
            is_editable = false
        }
    }
}
```

## Argument Reference

The following fields are supported:

* description
  (Optional):
  A description of the profile.

* label
  (Required):
  A user friendly label for the profile.

* name
  (Required):
  The name of the profile.

* namespace
  (Optional):
  The name of the namespace in which this resource belongs. If not provided, the default namespace is used.

* profile_provisioner
  (Required):
  The config of the provsioner to use for the profile.

* profile_provisioner.name
  (Required):
  The name of the provisioner.

* profile_provisioner.properties
  (Required):
  The properties of the provisioner.

* profile_provisioner.properties.is_editable
  (Required):
  Whether the value can be updated.

* profile_provisioner.properties.name
  (Required):
  The name of the property.

* profile_provisioner.properties.value
  (Required):
  The value of the property.


