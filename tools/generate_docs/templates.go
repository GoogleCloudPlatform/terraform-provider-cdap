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
* {{$name}}
  {{if $resource.Required -}}
  (Required):
  {{else if $resource.Optional -}}
  (Optional):
  {{else if $resource.Computed -}}
  (Computed):
  {{end -}}
  {{$resource.Description}}

{{end -}}
`))
)

type templateArgs struct {
	Title  string
	Schema map[string]*schema.Schema
}
