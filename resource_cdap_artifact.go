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
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/artifact.html
func resourceArtifact() *schema.Resource {
	return &schema.Resource{
		Create: resourceArtifactCreate,
		Read:   resourceArtifactRead,
		Delete: resourceArtifactDelete,
		Exists: resourceArtifactExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"version": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"extends": {
				Type: schema.TypeList,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				ForceNew: true,
			},
			"jar_binary_path": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceArtifactCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", config.namespace, "/artifacts", name)

	jar, err := os.Open(d.Get("jar_binary_path").(string))
	if err != nil {
		return err
	}
	defer jar.Close()

	req, err := http.NewRequest(http.MethodPost, addr, jar)
	if err != nil {
		return err
	}

	req.Header = map[string][]string{}
	if v, ok := d.GetOk("version"); ok {
		req.Header.Add("Artifact-Version", v.(string))
	}
	if v, ok := d.GetOk("extends"); ok {
		var es []string
		for _, e := range v.([]interface{}) {
			es = append(es, e.(string))
		}
		req.Header.Add("Artifact-Extends", strings.Join(es, "/"))
	}

	if _, err := httpCall(config.client, req); err != nil {
		return err
	}

	d.SetId(name)
	return nil
}

func resourceArtifactRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceArtifactDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", config.namespace, "/artifacts", name, "/versions", d.Get("version").(string))

	req, err := http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.client, req)
	return err
}

func resourceArtifactExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", config.namespace, "/artifacts")

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}

	b, err := httpCall(config.client, req)
	if err != nil {
		return false, err
	}

	type artifact struct {
		Name string `json:"name"`
	}

	var artifacts []artifact
	if err := json.Unmarshal(b, &artifacts); err != nil {
		return false, err
	}

	for _, a := range artifacts {
		if a.Name == name {
			return true, nil
		}
	}
	return false, nil
}
