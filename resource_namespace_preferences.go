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

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/lifecycle.html.
func resourceNamespacePreferences() *schema.Resource {
	return &schema.Resource{
		Create: resourceNamespacePreferencesCreate,
		Read:   resourceNamespacePreferencesRead,
		Delete: resourceNamespacePreferencesDelete,

		Schema: map[string]*schema.Schema{
			"namespace": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				DefaultFunc: func() (interface{}, error) {
					return defaultNamespace, nil
				},
			},
			"properties": {
				Type:     schema.TypeMap,
				Required: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceNamespacePreferencesCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	namespace := d.Get("namespace").(string)
	addr := urlJoin(config.host, "/v3/namespaces", namespace, "/preferences")

	b, err := json.Marshal(d.Get("properties"))
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

	d.SetId(namespace)
	return nil
}

func resourceNamespacePreferencesRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceNamespacePreferencesDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/preferences")

	req, err := http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.client, req)
	return err
}
