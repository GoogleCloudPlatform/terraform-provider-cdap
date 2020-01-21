# Artifact Example

This example shows how to automate deployment of an artifact (plugin). We will be deploying the
Trash plugin (version 1.2.0).

## Creating the resources

You need two items to be able create this resource:

- The artifact JAR binary.
- The artifact JSON configuration.

You can get these from either building it from source or from a pre-built location.

For the Trash plugin, you can build it from the source by cloning the
[Github repo](https://github.com/data-integrations/trash/tree/release/1.2) or by fetching it from
the Cloud Data Fusion
[Healthcare Hub](https://console.cloud.google.com/storage/browser/aeba5c94-db31-451a-85ea-27047cbe133b/packages/plugin-trash-sink/1.2.0/).

Now you can create your Terraform resources:

- Create one `cdap_artifact` resource. Set the name and version to match the ones used in the repo.
- Set the jar_binary_path to the JAR file downloaded above.
- Set the `extends` field in the `cdap_artifact` resource the same as the `parents` field in the
  config JSON.
- Create one `cdap_artifact_property` for each property in the `properties` field in the config
  JSON.
- You can now deploy the artifact by running `terraform apply`.

## Updates

The version used in the example may become outdated, so please use the latest version when deploying
to a production environment. You should also regularly update your plugins to the latest version.

To do an update, deploy the latest version in addition to the current version. Then stop all your
current pipelines that use the current version and create new pipelines with the new version. Then
start your new pipeline to proceed.
