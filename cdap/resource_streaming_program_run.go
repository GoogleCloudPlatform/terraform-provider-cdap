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

	runId, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("error generating uuid for faux run id: %s", err)
	}
	// This runtime arg will be unused by the pipeline but will allow the provider to associate a run with this resource.
	argsObj[fauxRunID] = runId.String()

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
		isRunning, err := isFauxRunIDRunningYet(config, runsAddr, runId.String())
		if err != nil {
			return resource.NonRetryableError(err)
		}
		if isRunning {
			d.SetId(runId.String())
			return nil
		}
		return resource.RetryableError(fmt.Errorf("still waiting for program run with faux run id: %s which is in an initializing state", runId.String()))
	})
}

//TODO ?
func resourceStreamingProgramRunRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

type parsedRuntimeArgs struct {
	FauxRunID string `json:"__FAUX_RUN_ID__"`
}

type runtimePropertiess struct {
	RuntimeArgs json.RawMessage `json:"runtimeArgs"`
}

// These are the only keys we need
type run struct {
	RunID      string `json:"runid"`
	Status     string `json:"status"`
	Properties runtimePropertiess
}

// Checks if there is a running run for the terraform faux run id
// raises error if the program is not in an initializing state (e.g. it failed or was killed in the ui)
func isFauxRunIDRunningYet(config *Config, runsAddr string, runId string) (bool, error) {
	s, err := getProgramRunStatusByFauxId(config, runsAddr, runId)
	if err != nil {
		return false, err
	}

	if s == "RUNNING" {
		return true, nil
	}
	if !programRunInitializingStatuses[s] {
		return false, fmt.Errorf("program not running or initializing, in state: %s", s)
	}
	return false, nil

}

func getProgramRunStatusByFauxId(config *Config, runsAddr string, runId string) (string, error) {
	r, err := getRunbyFauxID(config, runsAddr, runId)
	if err != nil {
		return "", err
	}

	return r.Status, nil
}

// TODO optimization: we call this function often when we could probably get the real runid once and cache it.
// This would avoid redoing this loop everytime to get the same result. probably inconsequential unless there are many runs of this program
func getRunbyFauxID(config *Config, runsAddr string, runId string) (r *run, err error) {
	req, err := http.NewRequest(http.MethodGet, runsAddr, nil)
	if err != nil {
		return
	}

	b, err := httpCall(config.httpClient, req)
	if err != nil {
		return
	}

	var runs []*run
	var args parsedRuntimeArgs

	err = json.Unmarshal(b, &runs)
	if err != nil {
		return &run{}, fmt.Errorf("could not unmarshal run payload: %s", err)
	}

	for _, r := range runs {
		// Unescaping runtime Args which are stored as an escaped JSON string.
		unquotedArgs, err := strconv.Unquote(string(r.Properties.RuntimeArgs))
		if err != nil {
			return &run{}, fmt.Errorf("error unescaping runtime arguments for run: %s: %s. runtime arguments was: %s", r.RunID, err, string(r.Properties.RuntimeArgs))
		}
		log.Printf("unquoted runtime args: %v", unquotedArgs)
		b := []byte(unquotedArgs)
		if err := json.Unmarshal(b, &args); err != nil {
			// This happens when a run does not contain the special faux id.
			log.Println("failed to parse unquotedArgs json")
		}
		log.Printf("found terraform faux run id: %s", args.FauxRunID)
		log.Printf("status: %s", r.Status)
		if runId == args.FauxRunID {
			return r, nil
		}
	}
	return &run{}, fmt.Errorf("no run found with faux runid: %s", runId)
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
	r, err := getRunbyFauxID(config, runsAddr, d.Id())

	if err != nil {
		return err
	}

	stopAddr := urlJoin(runsAddr, r.RunID, "/stop")

	return resource.Retry(d.Timeout(schema.TimeoutDelete), func() *resource.RetryError {
		s, err := getProgramRunStatusByFauxId(config, runsAddr, d.Id())
		if err != nil {
			return resource.NonRetryableError(fmt.Errorf("error getting program status by faux id: %s", err))
		}

		if s == "RUNNING" || programRunInitializingStatuses[s] {
			err = stopProgramRun(config, stopAddr)
			if err != nil {
				return resource.NonRetryableError(fmt.Errorf("error stopping program: %s", err))
			}
			time.Sleep(10 * time.Second)
			return resource.RetryableError(errors.New("Polling again to see if status progressed from RUNNING to an end status"))
		}

		if programRunEndStatuses[s] {
			return nil
		}

		return resource.NonRetryableError(fmt.Errorf("failed to delete run with id: %s and faux id: %s in status: %s", r.RunID, d.Id(), s))
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
		running, err := isFauxRunIDRunningYet(config, runAddr, d.Id())
		if err != nil {
			return false, fmt.Errorf("error determining status of run with FauxId %s", d.Id())
		}

		if running {
			return true, nil
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
		d.Get("program").(string))
}
