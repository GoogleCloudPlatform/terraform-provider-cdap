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
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/lifecycle.html.
func resourceStreamingProgram() *schema.Resource {
	return &schema.Resource{
		Create: resourceStreamingProgramCreate,
		Read:   resourceStreamingProgramRead,
		Delete: resourceStreamingProgramDelete,
		Exists: resourceStreamingProgramExists,

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
			"app": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the application.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the program.",
				DefaultFunc: func() (interface{}, error) {
					return "DataStreamsSparkStreaming", nil
				},
			},
			"type": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "One of flows, mapreduce, services, spark, workers, or workflows.",
				ValidateFunc: validation.StringInSlice([]string{"flows", "mapreduce", "services", "spark", "workers", "workflows"}, false),
				DefaultFunc: func() (interface{}, error) {
					return "spark", nil
				},
			},
			"runtime_arguments": {
				Type:        schema.TypeMap,
				Required:    true,
				ForceNew:    true,
				Description: "The runtime arguments used to start the program",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceStreamingProgramCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)

	addr := urlJoin(
		config.host,
		"/v3/namespaces", d.Get("namespace").(string),
		"/apps", d.Get("app").(string), d.Get("type").(string),
		d.Get("name").(string), "start")

	argsObj := make(map[string]interface{})

	b, err := json.Marshal(argsObj)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(b))
	if err != nil {
		return err
	}

	if _, err := httpCall(config.httpClient, req); err != nil {
		return err
	}

	d.SetId(name)
	return nil
}

func resourceStreamingProgramRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceStreamingProgramDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)

	addr := urlJoin(
		config.host,
		"/v3/namespaces", d.Get("namespace").(string),
		"/apps", d.Get("app").(string), d.Get("type").(string),
		name, "stop")

	req, err := http.NewRequest(http.MethodPost, addr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.httpClient, req)
	return err
}

func resourceStreamingProgramExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)

	addr := urlJoin(
		config.host,
		"/v3/namespaces", d.Get("namespace").(string),
		"/apps", d.Get("app").(string), d.Get("type").(string),
		d.Get("name").(string), "/status")

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}

	b, err := httpCall(config.httpClient, req)
	if err != nil {
		return false, err
	}

	type ProgramStatus struct {
		Status string `json:"status"`
	}

	var p ProgramStatus
	if err := json.Unmarshal(b, &p); err != nil {
		return false, err
	}

	if p.Status == "RUNNING" {
		return true, nil
	}
	return false, nil
}
