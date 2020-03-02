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
	"fmt"
	"log"
	"net/http"
	"time"

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
		name)
	startAddr := urlJoin(
		addr, "start")
	statusAddr := urlJoin(
		addr, "status")

	argsObj := make(map[string]interface{})

	b, err := json.Marshal(argsObj)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, startAddr, bytes.NewReader(b))
	if err != nil {
		return err
	}

	if _, err := httpCall(config.httpClient, req); err != nil {
		return err
	}

	// Poll until actually reaches RUNNING state.
	for {
		p, err := getProgramStatus(config, statusAddr)
		if err != nil {
			return err
		}

		switch s := p.Status; s {
		case "PROVISIONING":
			log.Println("program still in PROVISIONING state, waiting 10 seconds.")
			time.Sleep(10 * time.Second)
		case "STARTING":
			log.Println("program still in STARTING state, waiting 10 seconds.")
			time.Sleep(10 * time.Second)
		case "STOPPED":
			log.Println("program still in STOPPED state, waiting 10 seconds. This may occur when redeploying a pipeline.")
			time.Sleep(10 * time.Second)
		case "RUNNING":
			log.Println("program successfully reached RUNNING state.")
			return nil
        case "FAILED":
			return fmt.Errorf("failed to start program in app: %s in state: %s.", d.Get("app"), p.Status)
		default:
			log.Println("program still in STARTING state, waiting 10 seconds.")
			time.Sleep(10 * time.Second)
		}
	}

	d.SetId(d.Get("app").(string))
	return nil
}

func resourceStreamingProgramRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func getProgramStatus(config *Config, statusAddr string) (p ProgramStatus, err error) {
	req, err := http.NewRequest(http.MethodGet, statusAddr, nil)
	if err != nil {
		return
	}

	b, err := httpCall(config.httpClient, req)
	if err != nil {
		return
	}

	if err := json.Unmarshal(b, &p); err != nil {
		return p, err
	}

	return p, nil
}

func stopProgram(config *Config, stopAddr string) error {
	req, err := http.NewRequest(http.MethodPost, stopAddr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.httpClient, req)
	return err
}

func resourceStreamingProgramDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)

	addr := urlJoin(
		config.host,
		"/v3/namespaces", d.Get("namespace").(string),
		"/apps", d.Get("app").(string), d.Get("type").(string),
		name)

	// Check status to handle scenarios like a program that is in STOPPING state.
	statusAddr := urlJoin(
		addr, "/status")
	stopAddr := urlJoin(
		addr, "/stop")

	// Poll until actually reaches STOPPED state.
	for {
		p, err := getProgramStatus(config, statusAddr)
		if err != nil {
			return err
		}

		switch s := p.Status; s {
		case "STOPPED":
			return nil
		case "RUNNING":
			err = stopProgram(config, stopAddr)
			if err != nil {
				return err
			}
		case "STOPPING":
			log.Println("program still in STOPPING state, waiting 10 seconds.")
			time.Sleep(10 * time.Second)
		default:
			return fmt.Errorf("cannot stop program in app %s in state: %s", d.Get("app"), p.Status)
		}
	}
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

	var p ProgramStatus
	if err := json.Unmarshal(b, &p); err != nil {
		return false, err
	}

	if p.Status == "RUNNING" {
		return true, nil
	}
	return false, nil
}

type ProgramStatus struct {
	Status string `json:"status"`
}
