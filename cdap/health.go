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
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func chain(fs ...schema.CreateFunc) schema.CreateFunc {
	return schema.CreateFunc(func(d *schema.ResourceData, m interface{}) error {
		for _, f := range fs {
			if err := f(d, m); err != nil {
				return err
			}
		}
		return nil
	})
}

var retryErrCodes = map[int]bool{400: true, 502: true, 504: true}

// TODO(umairidris): Remove this once CDF create call returns only after all services are running.
func checkHealth(_ *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	for i := 0; i < 50; i++ {
		log.Printf("checking system artifact attempt %d", i)
		time.Sleep(10 * time.Second)
		exists, err := artifactExists(config, "cdap-data-pipeline", "default")
		var e *httpError
		switch {
		case errors.As(err, &e) && retryErrCodes[e.code]:
			log.Printf("checking for system artifacts got error code %v, retrying after 10 seconds", e.code)
		case err != nil:
			return fmt.Errorf("failed to check for aritfact existence: %v", err)
		case exists:
			log.Println("system artifact exists")
			return nil
		default: // !exists
			log.Println("system artifact not yet loaded, retrying after 10 seconds")
		}
	}
	return errors.New("system artifact failed to come up in 50 tries")
}
