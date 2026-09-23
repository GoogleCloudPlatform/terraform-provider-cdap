// Copyright 2026 Google LLC
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
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const wranglerDirectiveConfigPath = "/v3/namespaces/system/apps/dataprep/services/service/methods/config"

func resourceWranglerDirectiveConfig() *schema.Resource {
	return &schema.Resource{
		Create: resourceWranglerDirectiveConfigCreate,
		Read:   resourceWranglerDirectiveConfigRead,
		Delete: resourceWranglerDirectiveConfigDelete,
		Exists: resourceWranglerDirectiveConfigExists,

		Schema: map[string]*schema.Schema{
			"config": {
				Type:         schema.TypeString,
				Description:  "The JSON string for the DirectiveConfig.",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsJSON,
				StateFunc: func(v interface{}) string {
					json, _ := structure.NormalizeJsonString(v)
					return json
				},
			},
		},
	}
}

func resourceWranglerDirectiveConfigCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	addr := urlJoin(config.host, wranglerDirectiveConfigPath)

	body := strings.NewReader(d.Get("config").(string))

	req, err := http.NewRequest(http.MethodPost, addr, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	if _, err := httpCall(config, req); err != nil {
		return err
	}

	d.SetId("system")
	return nil
}

func resourceWranglerDirectiveConfigRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceWranglerDirectiveConfigDelete(d *schema.ResourceData, m interface{}) error {
	d.SetId("")
	return nil
}

func resourceWranglerDirectiveConfigExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)
	addr := urlJoin(config.host, wranglerDirectiveConfigPath)

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}

	_, err = httpCall(config, req)
	if err != nil {
		if hErr, ok := err.(*httpError); ok && hErr.code == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
