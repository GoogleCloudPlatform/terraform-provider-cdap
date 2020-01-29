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

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func generate(provider *schema.Provider, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	var buf bytes.Buffer
	args := templateArgs{Title: "CDAP Provider", Schema: provider.Schema}
	if err := markdownTemplate.Execute(&buf, args); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(outputDir, "provider.md"), buf.Bytes(), 0644); err != nil {
		return err
	}

	if len(provider.ResourcesMap) == 0 {
		return nil
	}

	resourcesDir := filepath.Join(outputDir, "r")
	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		return err
	}

	for name, res := range provider.ResourcesMap {
		var buf bytes.Buffer
		args := templateArgs{Title: name, Schema: flattenSchema(res.Schema)}
		if err := markdownTemplate.Execute(&buf, args); err != nil {
			return err
		}
		p := filepath.Join(resourcesDir, fmt.Sprintf("%s.md", name))
		if err := ioutil.WriteFile(p, buf.Bytes(), 0644); err != nil {
			return err
		}
	}
	return nil
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
