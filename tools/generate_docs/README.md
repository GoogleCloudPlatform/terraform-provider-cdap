# Docs Generator

The `generate_docs` tool helps automate building the provider website using
[Go templates](https://golang.org/pkg/text/template/).

See the [templates](./templates) folder which contains templates for common
elements as well as all resources.

To run the generator (from the root directory):

```bash
go build ./tools/generate_docs
./generate_docs --output_dir="./docs" --template_dir=tools/generate_docs/templates
```
