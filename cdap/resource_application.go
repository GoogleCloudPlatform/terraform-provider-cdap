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
	"github.com/hashicorp/terraform-plugin-sdk/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/lifecycle.html.
func resourceApplication() *schema.Resource {
	return &schema.Resource{
		Create: resourceApplicationCreate,
		Read:   resourceApplicationRead,
		Delete: resourceApplicationDelete,
		Exists: resourceApplicationExists,

		Schema: map[string]*schema.Schema{
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The name of the namespace in which this resource belongs. If not provided, the default namespace is used.",
				DefaultFunc: func() (interface{}, error) {
					return defaultNamespace, nil
				},
			},
			"name": {
				Type:        schema.TypeString,
				Description: "The name of the application. This will be used as the unique identifier in the CDAP API.",
				Required:    true,
				ForceNew:    true,
			},
			"spec": {
				Type:         schema.TypeString,
				Description:  "The full contents of the exported pipeline JSON spec.",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.ValidateJsonString,
				StateFunc: func(v interface{}) string {
					json, _ := structure.NormalizeJsonString(v)
					return json
				},
			},
		},
	}
}

func resourceApplicationCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/apps", name)

	body := strings.NewReader(d.Get("spec").(string))

	req, err := http.NewRequest(http.MethodPut, addr, body)
	if err != nil {
		return err
	}

	if _, err := httpCall(config.httpClient, req); err != nil {
		return err
	}

	d.SetId(name)
	return nil
}

func resourceApplicationRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceApplicationDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/apps", name)

	req, err := http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.httpClient, req)
	return err
}

func resourceApplicationExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/apps")
	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}

	b, err := httpCall(config.httpClient, req)
	if err != nil {
		return false, err
	}

	type app struct {
		Name string `json:"name"`
	}

	var apps []app
	if err := json.Unmarshal(b, &apps); err != nil {
		return false, err
	}

	for _, a := range apps {
		if a.Name == name {
			return true, nil
		}
	}
	return false, nil
}
