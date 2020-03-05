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
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

// This is a special key terraform will use to check for the existence of this run related to a particular resource
const FAUX_RUN_ID string = "__FAUX_RUN_ID__"

// https://github.com/cdapio/cdap/blob/1d62163faaecb5b888f4bccd0fcf4a8d27bbd549/cdap-proto/src/main/java/io/cdap/cdap/proto/ProgramRunStatus.java
var INITIALIZING_PROGRAM_RUN_STATUSES map[string]bool = map[string]bool{"PENDING": true, "STARTING": true}
var UNSUCCESSFUL_PROGRAM_RUN_STATUSES map[string]bool = map[string]bool{"FAILED": true, "KILLED": true, "REJECTED": true}
var END_PROGRAM_STATUSES map[string]bool = map[string]bool{"COMPLETED": true, "FAILED": true, "KILLED": true, "REJECTED": true}

// https://docs.cdap.io/cdap/current/en/reference-manual/http-restful-api/lifecycle.html.
func resourceStreamingProgramRun() *schema.Resource {
	return &schema.Resource{
		Create: resourceStreamingProgramRunCreate,
		Read:   resourceStreamingProgramRunRead,
		Delete: resourceStreamingProgramRunDelete,
		Exists: resourceStreamingProgramRunExists,

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
			"allow_multiple_runs": {
				Type:        schema.TypeBool,
				Required:    true,
				ForceNew:    true,
				Description: "Specifies if multiple runs of the same program should be allowed",
				DefaultFunc: func() (interface{}, error) {
					return false, nil
				},
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(20 * time.Minute),
			Delete: schema.DefaultTimeout(time.Hour), // This gives the pipeline time to drain processing if in-flight records
		},
	}
}

func resourceStreamingProgramRunCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)

	addr := getProgramAddr(config, d)
	startAddr := urlJoin(addr, "start")
	runsAddr := urlJoin(addr, "runs")

	argsObj := make(map[string]string)
	// cast map[string]interface{} to map[string]string
	for k, val := range d.Get("runtime_arguments").(map[string]interface{}) {
		argsObj[k] = val.(string)
	}

	runId, _ := uuid.NewRandom()
	// This runtime arg will be unused by the pipeline but will allow the provider to associate a run with this resource.
	argsObj[FAUX_RUN_ID] = runId.String()

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
	return resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		isRunning, err := isFauxRunIdRunningYet(config, runsAddr, runId.String())
		if err != nil {
			return resource.RetryableError(err)
		}
		if isRunning {
			d.SetId(runId.String())
			return nil
		}
		time.Sleep(10 * time.Second) // avoid spamming retries
		return resource.RetryableError(fmt.Errorf("still waiting for program run with faux run id: %s which is in an initializing state", runId.String()))
	})
}

//TODO ?
func resourceStreamingProgramRunRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

type RuntimeArgs struct {
	FauxRunId string `json:"__FAUX_RUN_ID__"`
}

type RuntimeProperties struct {
	RuntimeArgs json.RawMessage `json:"runtimeArgs"`
}

// These are the only keys we need
type Run struct {
	RunId      string `json:"runid"`
	Status     string `json:"status"`
	Properties RuntimeProperties
}

// Checks if there is a running run for the terraform faux run id
// raises error if the program is not in an initializing state (e.g. it failed or was killed in the ui)
func isFauxRunIdRunningYet(config *Config, runsAddr string, runId string) (bool, error) {
	s, err := getProgramRunStatusByFauxId(config, runsAddr, runId)
	if err != nil {
		return false, err
	}

	if s == "RUNNING" {
		return true, nil
	}
	if !INITIALIZING_PROGRAM_RUN_STATUSES[s] {
		return false, fmt.Errorf("Program not Running or Initializing, in state: %s", s)
	}
	return false, nil

}

func getProgramRunStatusByFauxId(config *Config, runsAddr string, runId string) (s string, err error) {
	r, err := getRunByFauxId(config, runsAddr, runId)
	if err != nil {
		return
	}

	return r.Status, nil
}

// TODO optimization: we call this function often when we could probably get the real runid once and cache it.
// This would avoid redoing this loop everytime to get the same result. probably inconsequential unless there are many runs of this program
func getRunByFauxId(config *Config, runsAddr string, runId string) (r Run, err error) {
	req, err := http.NewRequest(http.MethodGet, runsAddr, nil)
	if err != nil {
		return
	}

	b, err := httpCall(config.httpClient, req)
	if err != nil {
		return
	}

	var runs []Run
	var args RuntimeArgs

	err = json.Unmarshal(b, &runs)
	if err != nil {
		return
	}

	for _, r := range runs {
		// Unescaping runtime Args which are stored as an escaped JSON string.
		unquotedArgs, _ := strconv.Unquote(string(r.Properties.RuntimeArgs))
		log.Println(fmt.Sprintf("%s", unquotedArgs))
		b := []byte(unquotedArgs)
		if err := json.Unmarshal(b, &args); err != nil {
			// This happens when a run does not contain the special TF con
			log.Println("failed to parse unquotedArgs json")
		}
		log.Println(fmt.Sprintf("found terraform faux run id: %s", args.FauxRunId))
		log.Println(fmt.Sprintf("status: %s", r.Status))
		if runId == args.FauxRunId {
			return r, nil
		}
	}
	err = fmt.Errorf("no run found with faux runid: %s", runId)
	return
}

func stopProgramRun(config *Config, stopAddr string) error {
	req, err := http.NewRequest(http.MethodPost, stopAddr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.httpClient, req)
	return err
}

func resourceStreamingProgramRunDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)

	addr := getProgramAddr(config, d)
	runsAddr := urlJoin(addr, "/runs")
	r, err := getRunByFauxId(config, runsAddr, d.Id())

	if err != nil {
		return err
	}

	stopAddr := urlJoin(runsAddr, r.RunId, "/stop")

	return resource.Retry(d.Timeout(schema.TimeoutDelete), func() *resource.RetryError {
		s, err := getProgramRunStatusByFauxId(config, runsAddr, d.Id())

		if s == "RUNNING" || INITIALIZING_PROGRAM_RUN_STATUSES[s] {
			err = stopProgramRun(config, stopAddr)
			if err != nil {
				return resource.NonRetryableError(fmt.Errorf("error stopping program: %s", err))
			}
		}

		if END_PROGRAM_STATUSES[s] {
			return nil
		}

		return resource.NonRetryableError(fmt.Errorf("failed to delete run with id: %s and faux id: %s in status: %s", r.RunId, d.Id(), s))
	})
}

func resourceStreamingProgramRunExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)

	addr := getProgramAddr(config, d)
	statusAddr := urlJoin(addr, "/status")
	runAddr := urlJoin(addr, "/runs")

	req, err := http.NewRequest(http.MethodGet, statusAddr, nil)
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

	// This checks if there program is running (but it may be running several times)
	if p.Status == "RUNNING" {
		// This handles ambiguity if there are multiple program runs
		running, err := isFauxRunIdRunningYet(config, runAddr, d.Id())
		if err != nil { // We
			return false, nil
		}

		if running {
			return true, nil
		}

		if d.Get("allow_multiple_runs").(bool) {
			return false, nil
		} else {
			return true, errors.New("there is a RUNNING run of this program and allow_multiple_runs is false")
		}
	}

	return false, nil
}

type ProgramStatus struct {
	Status string `json:"status"`
}

func getProgramAddr(config *Config, d *schema.ResourceData) string {
	return urlJoin(
		config.host,
		"/v3/namespaces", d.Get("namespace").(string),
		"/apps", d.Get("app").(string), d.Get("type").(string),
		d.Get("name").(string))
}
