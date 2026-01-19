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
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceOAuthCredentialValidation() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceOAuthCredentialValidationRead,
		Schema: map[string]*schema.Schema{
			"oauth_provider": {
				Type:     schema.TypeString,
				Required: true,
			},
			"credential_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"is_valid": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func dataSourceOAuthCredentialValidationRead(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	provider := d.Get("oauth_provider").(string)
	credID := d.Get("credential_id").(string)

	// Append "/valid" to the path
	basePath := fmt.Sprintf(oauthCredentialBasePath, provider)
	addr := urlJoin(config.host, basePath, credID, "valid")

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	respBody, err := httpCall(config, req)
	if err != nil {
		return fmt.Errorf("failed to check oauth credential validity: %v", err)
	}

	var validResp CredentialIsValidResponse
	if err := json.Unmarshal(respBody, &validResp); err != nil {
		return fmt.Errorf("failed to decode validity response: %v", err)
	}

	d.Set("is_valid", validResp.Valid)
	// Unique ID for the data source state
	d.SetId(fmt.Sprintf("%s:%s:validation", provider, credID))

	return nil
}
