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
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/lifecycle.html.
func resourceApplication() *schema.Resource {
	return &schema.Resource{
		Create: resourceApplicationCreate,
		Read:   resourceApplicationRead,
		Delete: resourceApplicationDelete,
		Exists: resourceApplicationExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the application.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The name of the namespace in which this resource belongs. If not provided, the default namespace is used.",
				DefaultFunc: func() (interface{}, error) {
					return defaultNamespace, nil
				},
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "A user friendly description of the application.",
			},
			"artifact": {
				Type:        schema.TypeList,
				Required:    true,
				ForceNew:    true,
				Description: "The artifact used to create the pipeline",
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							ForceNew:    true,
							Description: "The name of the artifact.",
						},
						"version": {
							Type:        schema.TypeString,
							Required:    true,
							ForceNew:    true,
							Description: "The version of the artifact.",
						},
						"scope": {
							Type:        schema.TypeString,
							Optional:    true,
							ForceNew:    true,
							Description: "The scope of the artifact, one of either SYSTEM or USER. Defaults to SYSTEM.",
							DefaultFunc: func() (interface{}, error) {
								return "SYSTEM", nil
							},
						},
					},
				},
			},
			"config": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The JSON encoded configuration of the pipeline",
			},
		},
	}
}

func resourceApplicationCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)

	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/apps", name)

	confObj := make(map[string]interface{})
	if err := json.Unmarshal([]byte(d.Get("config").(string)), &confObj); err != nil {
		return err
	}

	obj := map[string]interface{}{
		"name":     name,
		"artifact": d.Get("artifact").([]interface{})[0],
		"config":   confObj,
	}

	if v, ok := d.GetOk("description"); ok {
		obj["description"] = v
	}

	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, addr, bytes.NewReader(b))
	if err != nil {
		return err
	}

	if _, err := httpCall(config.client, req); err != nil {
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
	_, err = httpCall(config.client, req)
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

	b, err := httpCall(config.client, req)
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
