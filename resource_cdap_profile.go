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
	"encoding/json"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/profile.html
func resourceProfile() *schema.Resource {
	return &schema.Resource{
		Create: resourceProfileCreate,
		Read:   resourceProfileRead,
		Delete: resourceProfileDelete,
		Exists: resourceProfileExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"label": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"profile_provisioner": {
				Type:     schema.TypeList,
				Required: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
						},
						"properties": {
							Type:     schema.TypeList,
							Required: true,
							ForceNew: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:     schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"value": {
										Type:     schema.TypeString,
										Required: true,
										ForceNew: true,
									},
									"is_editable": {
										Type:     schema.TypeBool,
										Optional: true,
										ForceNew: true,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

type profile struct {
	Name        string       `json:"name,omitempty"`
	Label       string       `json:"label"`
	Description string       `json:"description,omitempty"`
	Provisioner *provisioner `json:"provisioner"`
}

type provisioner struct {
	Name       string      `json:"name"`
	Properties []*property `json:"properties"`
}

type property struct {
	Name       string `json:"name"`
	Value      string `json:"value"`
	IsEditable bool   `json:"isEditable"`
}

func resourceProfileCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)

	prof := &profile{
		Label:       d.Get("label").(string),
		Description: d.Get("description").(string),
	}

	rawProv := d.Get("profile_provisioner").([]interface{})[0].(map[string]interface{})
	prov := &provisioner{
		Name: rawProv["name"].(string),
	}
	for _, rawProp := range rawProv["properties"].([]interface{}) {
		rawPropMap := rawProp.(map[string]interface{})
		prov.Properties = append(prov.Properties, &property{
			Name:       rawPropMap["name"].(string),
			Value:      rawPropMap["value"].(string),
			IsEditable: rawPropMap["is_editable"].(bool),
		})
	}
	prof.Provisioner = prov

	addr := urlJoin(config.host, "/v3/namespaces", config.namespace, "/profiles", name)

	b, err := json.Marshal(prof)
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

func resourceProfileRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceProfileDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)

	addr := urlJoin(config.host, "/v3/namespaces", config.namespace, "/profiles", name)

	// Disable the profile first.
	req, err := http.NewRequest(http.MethodPost, urlJoin(addr, "/disable"), nil)
	if err != nil {
		return err
	}
	if _, err := httpCall(config.client, req); err != nil {
		return err
	}

	req, err = http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.client, req)
	return err
}

func resourceProfileExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", config.namespace, "/profiles")

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}

	b, err := httpCall(config.client, req)
	if err != nil {
		return false, err
	}

	var profiles []profile
	if err := json.Unmarshal(b, &profiles); err != nil {
		return false, err
	}

	for _, p := range profiles {
		if p.Name == name {
			return true, nil
		}
	}
	return false, nil
}
