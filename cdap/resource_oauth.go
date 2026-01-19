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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const oauthProviderBasePath = "v3/namespaces/system/apps/pipeline/services/studio/methods/v1/oauth/provider"

// OAuthProviderRequest represents Data body for Put Request OAuth Provider
type OAuthProviderRequest struct {
	ClientId                   string `json:"clientId"`
	ClientSecret               string `json:"clientSecret"`
	LoginUrl                   string `json:"loginURL"`
	TokenRefreshUrl            string `json:"tokenRefreshURL"`
	CredentialEncodingStrategy string `json:"credentialEncodingStrategy,omitempty"`
	UserAgent                  string `json:"userAgent,omitempty"`
}

func resourceOAuthProvider() *schema.Resource {
	return &schema.Resource{
		Create: resourceOAuthProviderCreate,
		Read:   resourceOAuthProviderRead,
		Update: resourceOAuthProviderUpdate,
		Delete: resourceOAuthProviderDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true, // The provider name is in the URL path, so changing it requires a new resource
			},
			"client_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"client_secret": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true, // Terraform will hide this in the terminal output
			},
			"login_url": {
				Type:     schema.TypeString,
				Required: true,
			},
			"token_refresh_url": {
				Type:     schema.TypeString,
				Required: true,
			},
			"credential_encoding_strategy": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"BASIC_AUTH", "FORM_BODY"}, false),
			},
			"user_agent": {
				Type:     schema.TypeString,
				Optional: true,
			},
		},
	}
}

func resourceOAuthProviderCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Get("name").(string)

	payload := OAuthProviderRequest{
		ClientId:                   d.Get("client_id").(string),
		ClientSecret:               d.Get("client_secret").(string),
		LoginUrl:                   d.Get("login_url").(string),
		TokenRefreshUrl:            d.Get("token_refresh_url").(string),
		CredentialEncodingStrategy: d.Get("credential_encoding_strategy").(string),
		UserAgent:                  d.Get("user_agent").(string),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode oauth provider json: %v", err)
	}

	addr := urlJoin(config.host, oauthProviderBasePath, name)

	req, err := http.NewRequest(http.MethodPut, addr, bytes.NewReader(body))
	if err != nil {
		return err
	}

	if _, err := httpCall(config, req); err != nil {
		return err
	}

	d.SetId(name)
	return nil
}

func resourceOAuthProviderUpdate(d *schema.ResourceData, m interface{}) error {
	// PUT overwrites in CDAP, so Update delegates to Create logic
	return resourceOAuthProviderCreate(d, m)
}

func resourceOAuthProviderDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	name := d.Id()
	addr := urlJoin(config.host, oauthProviderBasePath, name)

	req, err := http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}

	_, err = httpCall(config, req)
	if err != nil {
		d.SetId("")
		return err
	}
	return nil
}

func resourceOAuthProviderRead(d *schema.ResourceData, m interface{}) error {
	return nil
}
