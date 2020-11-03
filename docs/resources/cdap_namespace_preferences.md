<!-- AUTO GENERATED CODE. DO NOT EDIT MANUALLY. -->
# cdap_namespace_preferences


# Example

```
resource "cdap_namespace_preferences" "preferences" {
  namespace   = "example"
  preferences = {
    FOO = "BAR"
  }
}
```

## Argument Reference

The following fields are supported:

* namespace
  (Optional):
  The name of the namespace in which this resource belongs. If not provided, the default namespace is used.

* preferences
  (Required):
  The preferences to set on the namespace.


