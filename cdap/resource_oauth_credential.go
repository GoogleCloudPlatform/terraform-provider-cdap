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
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// API Path constant for credentials
const oauthCredentialBasePath = "v3/namespaces/system/apps/pipeline/services/studio/methods/v1/oauth/provider/%s/credential"

// Structs for JSON Payloads
type PutOAuthCredentialRequest struct {
	OneTimeCode string `json:"oneTimeCode"`
	RedirectURI string `json:"redirectURI"`
}

type GetAccessTokenResponse struct {
	AccessToken string `json:"accessToken"`
	InstanceURL string `json:"instanceURL"`
}

type CredentialIsValidResponse struct {
	Valid bool `json:"valid"`
}

func resourceOAuthCredential() *schema.Resource {
	return &schema.Resource{
		Create: resourceOAuthCredentialCreate,
		Read:   resourceOAuthCredentialRead,
		Delete: resourceOAuthCredentialDelete,

		Schema: map[string]*schema.Schema{
			"oauth_provider": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true, // Changing provider implies a totally new credential
			},
			"credential_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true, // The ID in the URL path
			},
			"one_time_code": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true, // A code is one-time use; changing it requires re-exchange (re-creation)
			},
			"redirect_uri": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"access_token": {
				Type:      schema.TypeString,
				Computed:  true,
				Sensitive: true,
			},
			"is_valid": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func resourceOAuthCredentialCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	provider := d.Get("oauth_provider").(string)
	credID := d.Get("credential_id").(string)

	// 1. Prepare payload for PUT (Exchange Code)
	payload := PutOAuthCredentialRequest{
		OneTimeCode: d.Get("one_time_code").(string),
		RedirectURI: d.Get("redirect_uri").(string),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode oauth credential json: %v", err)
	}

	// Construct URL: .../provider/{provider}/credential/{credential}
	basePath := fmt.Sprintf(oauthCredentialBasePath, provider)
	addr := urlJoin(config.host, basePath, credID)

	// 2. Perform PUT Request
	req, err := http.NewRequest(http.MethodPut, addr, bytes.NewReader(body))
	if err != nil {
		return err
	}

	if _, err := httpCall(config, req); err != nil {
		return fmt.Errorf("failed to create oauth credential: %v", err)
	}

	d.SetId(credID)
	// 3. Immediately Read back the state (token & validity)
	return resourceOAuthCredentialRead(d, m)
}

func resourceOAuthCredentialRead(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	provider := d.Get("oauth_provider").(string)
	credID := d.Id() // Use d.Id() to get the stored credential ID

	basePath := fmt.Sprintf(oauthCredentialBasePath, provider)
	addr := urlJoin(config.host, basePath, credID)

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return err
	}

	respBody, err := httpCall(config, req)
	if err != nil {
		// If 404, remove from state
		if err.Error() == "404" {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read oauth credential: %v", err)
	}

	var tokenResp GetAccessTokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return fmt.Errorf("failed to decode access token response: %v", err)
	}

	if err := d.Set("access_token", tokenResp.AccessToken); err != nil {
		return err
	}

	validAddr := urlJoin(config.host, basePath, credID, "valid")
	reqValid, err := http.NewRequest(http.MethodGet, validAddr, nil)
	if err != nil {
		return err
	}

	validBody, err := httpCall(config, reqValid)
	if err != nil {
		return fmt.Errorf("failed to read oauth validity: %v", err)
	}

	var validResp CredentialIsValidResponse
	if err := json.Unmarshal(validBody, &validResp); err != nil {
		return fmt.Errorf("failed to decode validity response: %v", err)
	}

	if err := d.Set("is_valid", validResp.Valid); err != nil {
		return err
	}

	return nil
}

func resourceOAuthCredentialUpdate(d *schema.ResourceData, m interface{}) error {
	return resourceOAuthCredentialCreate(d, m)
}

func resourceOAuthCredentialDelete(d *schema.ResourceData, m interface{}) error {
	d.SetId("")
	return nil
}
