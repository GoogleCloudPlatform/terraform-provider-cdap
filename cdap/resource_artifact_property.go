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

package cdap

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/artifact.html#set-an-artifact-property.
func resourceArtifactProperty() *schema.Resource {
	return &schema.Resource{
		Create: resourceArtifactPropertyCreate,
		Read:   resourceArtifactPropertyRead,
		Delete: resourceArtifactPropertyDelete,
		Exists: resourceArtifactPropertyExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				DefaultFunc: func() (interface{}, error) {
					return defaultNamespace, nil
				},
			},
			"artifact_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"artifact_version": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"value": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceArtifactPropertyCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", d.Get("artifact_name").(string), "/versions", d.Get("artifact_version").(string), "/properties", name)

	req, err := http.NewRequest(http.MethodPut, addr, strings.NewReader(d.Get("value").(string)))
	if err != nil {
		return err
	}

	if _, err := httpCall(config.client, req); err != nil {
		return err
	}

	d.SetId(name)
	return nil
}

func resourceArtifactPropertyRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceArtifactPropertyDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", d.Get("artifact_name").(string), "/versions", d.Get("artifact_version").(string), "/properties", name)

	req, err := http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.client, req)
	return err
}

func resourceArtifactPropertyExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", d.Get("artifact_name").(string), "/versions", d.Get("artifact_version").(string), "/properties")

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}

	b, err := httpCall(config.client, req)
	if err != nil {
		return false, err
	}

	aps := make(map[string]string)
	if err := json.Unmarshal(b, &aps); err != nil {
		return false, err
	}

	_, ok := aps[name]
	return ok, nil
}
