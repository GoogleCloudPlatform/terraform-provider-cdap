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
const fauxRunID = "__FAUX_RUN_ID__"

// https://github.com/cdapio/cdap/blob/1d62163faaecb5b888f4bccd0fcf4a8d27bbd549/cdap-proto/src/main/java/io/cdap/cdap/proto/ProgramRunStatus.java
var (
	programRunInitializingStatuses = map[string]bool{"PENDING": true, "STARTING": true}
	programRunUnsuccessfulStatuses = map[string]bool{"FAILED": true, "KILLED": true, "REJECTED": true}
	programRunEndStatuses          = map[string]bool{"COMPLETED": true, "FAILED": true, "KILLED": true, "REJECTED": true}
)

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
			"program": {
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
			"run_id": {
				Type:        schema.TypeString,
				Computed:    true,
				ForceNew:    true,
				Description: "The run the CDAP Run ID",
				Elem:        &schema.Schema{Type: schema.TypeString},
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

	randomID, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("error generating uuid for faux run id: %v", err)
	}
	// This runtime arg will be unused by the pipeline but will allow the provider to associate a run with this resource.
	argsObj[fauxRunID] = randomID.String()

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
		time.Sleep(10 * time.Second) // avoid spamming retries and initial failure to find run.
		r, err := getRunByFauxID(config, runsAddr, randomID.String())
		if err != nil {
			return resource.NonRetryableError(err)
		}

		isRunning, err := isRunIDRunningYet(config, runsAddr, r.RunID)
		if err != nil {
			return resource.NonRetryableError(err)
		}

		if isRunning {
			d.Set("run_id", r.RunID)
			d.SetId(r.RunID)
			return nil
		}
		return resource.RetryableError(fmt.Errorf("still waiting for program run with id: %v which is in an initializing state", r.RunID))
	})
}

func resourceStreamingProgramRunRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

type runtimeArgs struct {
	FauxRunID string `json:"__FAUX_RUN_ID__"`
}

type runtimeProperties struct {
	RuntimeArgs *runtimeArgs `json:"runtimeArgs"`
}

func (ra *runtimeArgs) UnmarshalJSON(data []byte) error {
	unquoted, err := strconv.Unquote(string(data))
	if err != nil {
		return fmt.Errorf("failed to escape runtime arguments %v: %v", string(data), err)
	}

	// Use alias to avoid infinite recursion: https://stackoverflow.com/q/52433467
	type alias runtimeArgs
	var a alias
	if err := json.Unmarshal([]byte(unquoted), &a); err != nil {
		return err
	}
	*ra = runtimeArgs(a)
	return nil
}

// These are the only keys we need
type run struct {
	RunID      string            `json:"runid"`
	Status     string            `json:"status"`
	Properties runtimeProperties `json:"properties"`
}

// Checks if there is a running run for the terraform faux run id
// raises error if the program is not in an initializing state (e.g. it failed or was killed in the ui)
func isRunIDRunningYet(config *Config, runsAddr string, runID string) (bool, error) {
	r, err := getRunByID(config, runsAddr, runID)
	if err != nil {
		return false, fmt.Errorf("couldn't get run id: %v: %v", runID, err)
	}

	if r.Status == "RUNNING" {
		return true, nil
	}
	if !programRunInitializingStatuses[r.Status] {
		return false, fmt.Errorf("program not running or initializing, in state: %v", r.Status)
	}
	return false, nil
}

func getRunByFauxID(config *Config, runsAddr string, fauxRunID string) (*run, error) {
	req, err := http.NewRequest(http.MethodGet, runsAddr, nil)
	if err != nil {
		return nil, err
	}

	b, err := httpCall(config.httpClient, req)
	if err != nil {
		return nil, err
	}

	var runs []*run

	if err = json.Unmarshal(b, &runs); err != nil {
		return nil, fmt.Errorf("could not unmarshal run payload: %v", err)
	}

	for _, r := range runs {
		args := r.Properties.RuntimeArgs
		log.Printf("found terraform run id: %v faux run id: %v status: %v", r.RunID, args.FauxRunID, r.Status)
		if fauxRunID == args.FauxRunID {
			return r, nil
		}
	}
	return nil, fmt.Errorf("no run found with faux runid: %v", fauxRunID)
}

func getRunByID(config *Config, runsAddr string, runID string) (*run, error) {
	req, err := http.NewRequest(http.MethodGet, urlJoin(runsAddr, runID), nil)
	if err != nil {
		return nil, err
	}

	b, err := httpCall(config.httpClient, req)
	if err != nil {
		return nil, fmt.Errorf("couldn't retrived run with run id: %v: %s", runID, err)
	}

	var r *run

	if err = json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("could not unmarshal run payload: %v", err)
	}

	return r, nil
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
	stopAddr := urlJoin(runsAddr, d.Id(), "/stop")

	return resource.Retry(d.Timeout(schema.TimeoutDelete), func() *resource.RetryError {
		r, err := getRunByID(config, runsAddr, d.Id())
		if err != nil {
			return resource.NonRetryableError(fmt.Errorf("error getting program status by faux id: %v", err))
		}

		if r.Status == "RUNNING" || programRunInitializingStatuses[r.Status] {
			err = stopProgramRun(config, stopAddr)
			if err != nil {
				return resource.NonRetryableError(fmt.Errorf("error stopping program: %v", err))
			}
			time.Sleep(10 * time.Second)
			return resource.RetryableError(errors.New("Polling again to see if status progressed from RUNNING to an end status"))
		}

		if programRunEndStatuses[r.Status] {
			return nil
		}

		return resource.NonRetryableError(fmt.Errorf("failed to delete run with id: %v and faux id: %v in status: %v", r.RunID, d.Id(), r.Status))
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

	var p programStatus
	if err := json.Unmarshal(b, &p); err != nil {
		return false, err
	}

	running := false
	// This checks if the program is running (but it may be running several times)
	if p.Status == "RUNNING" {
		// This handles ambiguity if there are multiple program runs
		running, err = isRunIDRunningYet(config, runAddr, d.Id())
		if err != nil {
			return false, fmt.Errorf("error determining status of run with FauxId %v", d.Id())
		}

		return running, nil
	}

	return false, nil
}

type programStatus struct {
	Status string `json:"status"`
}

func getProgramAddr(config *Config, d *schema.ResourceData) string {
	return urlJoin(
		config.host,
		"/v3/namespaces", d.Get("namespace").(string),
		"/apps", d.Get("app").(string), d.Get("type").(string),
		d.Get("program").(string))
}
