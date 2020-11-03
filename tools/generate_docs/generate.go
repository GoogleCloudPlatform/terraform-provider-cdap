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
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"text/template"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func generate(provider *schema.Provider, tmplDir, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	tmpl, err := template.New("index.md.tmpl").ParseFiles(templateFiles(tmplDir, "index.md.tmpl")...)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	args := map[string]interface{}{
		"Schema": provider.Schema,
	}
	if err := tmpl.Execute(&buf, args); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(outputDir, "index.md"), buf.Bytes(), 0644); err != nil {
		return err
	}

	if len(provider.ResourcesMap) == 0 {
		return nil
	}

	resourcesOutputDir := filepath.Join(outputDir, "resources")
	if err := os.MkdirAll(resourcesOutputDir, 0755); err != nil {
		return err
	}

	for name, res := range provider.ResourcesMap {
		tmplName := fmt.Sprintf("%s.md.tmpl", name)
		tmpl, err := template.New(tmplName).ParseFiles(templateFiles(tmplDir, "resources/"+tmplName)...)
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		args := map[string]interface{}{
			"Title":  name,
			"Schema": flattenSchema(res.Schema),
		}
		if err := tmpl.Execute(&buf, args); err != nil {
			return err
		}
		p := filepath.Join(resourcesOutputDir, fmt.Sprintf("%s.md", name))
		if err := ioutil.WriteFile(p, buf.Bytes(), 0644); err != nil {
			return err
		}
	}
	return nil
}

func templateFiles(dir string, files ...string) []string {
	helpers := []string{
		"header.md.tmpl",
		"schema.md.tmpl",
	}

	var paths []string
	for _, t := range append(files, helpers...) {
		paths = append(paths, filepath.Join(dir, t))
	}
	return paths
}

func flattenSchema(schemas map[string]*schema.Schema) map[string]*schema.Schema {
	res := make(map[string]*schema.Schema)

	for name, sc := range schemas {
		res[name] = sc
		if sc.Elem == nil {
			continue
		}
		if resource, ok := sc.Elem.(*schema.Resource); ok {
			for subName, subSc := range flattenSchema(resource.Schema) {
				subName = name + "." + subName
				res[subName] = subSc
			}
		}
	}
	return res
}
