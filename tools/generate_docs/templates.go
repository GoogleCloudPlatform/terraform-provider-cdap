package main

import (
	"text/template"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

var (
	markdownTemplate = template.Must(template.New("provider").Parse(`
# {{.Title}}

## Argument Reference

The following fields are supported:

{{range $name $resource := .Schema}}
* {{$name}}:
  {{if $resource.Required}}
  (Required)
  {{elif $resource.Optional}}
  (Optional)
  {{end}}
  {{$resource.Description}}
{{end}}
`))
)

type templateArgs struct {
	Title  string
	Schema map[string]*schema.Schema
}
