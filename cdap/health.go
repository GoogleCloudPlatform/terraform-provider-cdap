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
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func wrap(fs ...schema.CreateFunc) schema.CreateFunc {
	return schema.CreateFunc(func(d *schema.ResourceData, m interface{}) error {
		for _, f := range fs {
			if err := f(d, m); err != nil {
				return err
			}
		}
		return nil
	})
}

// TODO(umairidris): Remove this once CDF create call returns only after all services are running.
func checkHealth(_ *schema.ResourceData, m interface{}) error {
	c := m.(*Config)

	var bad []string
	for i := 0; i < 10; i++ {
		serviceToStatus, err := getServiceToStatus(c)
		if err != nil {
			return err
		}
		if len(serviceToStatus) == 0 {
			return errors.New("found no services on instance")
		}

		bad = nil
		for service, status := range serviceToStatus {
			if status != "OK" {
				bad = append(bad, service)
			}
		}
		if len(bad) == 0 {
			log.Printf("Instance is healthy: %v", serviceToStatus)
			return nil
		}
		fmt.Printf("Found %v unhealthy services: %v", len(bad), bad)
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("found %v unhealthy services: %v", len(bad), bad)
}

func getServiceToStatus(c *Config) (map[string]string, error) {
	addr := urlJoin(c.host, "/v3/system/services/status")
	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return nil, err
	}
	b, err := httpCall(c.httpClient, req)
	if err != nil {
		return nil, err
	}

	var m map[string]string
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v\n%v", err, string(b))
	}
	return m, nil
}
