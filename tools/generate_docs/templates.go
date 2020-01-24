package main

import (
	"text/template"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

var (
	markdownTemplate = template.Must(template.New("provider").Parse(`# {{.Title}}

## Argument Reference

The following fields are supported:

{{range $name, $resource := .Schema -}}
* {{$name}}:
  {{if $resource.Required -}}
  (Required)
  {{else if $resource.Optional -}}
  (Optional)
  {{else if $resource.Computed -}}
  (Computed)
  {{end -}}
  {{$resource.Description}}

{{end -}}
`))
)

type templateArgs struct {
	Title  string
	Schema map[string]*schema.Schema
}
