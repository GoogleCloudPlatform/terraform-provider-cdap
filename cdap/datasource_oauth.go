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
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceOAuthURL() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceOAuthURLRead,

		Schema: map[string]*schema.Schema{
			"oauth_provider": {
				Type:     schema.TypeString,
				Required: true,
			},
			"redirect_uri": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"redirect_url": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"url": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceOAuthURLRead(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	providerName := d.Get("oauth_provider").(string)

	redirectURI := d.Get("redirect_uri").(string)
	redirectURL := d.Get("redirect_url").(string)

	addr := urlJoin(config.host, oauthProviderBasePath, providerName, "authurl")

	u, err := url.Parse(addr)
	if err != nil {
		return fmt.Errorf("error parsing base url %q due to: %v", addr, err)
	}

	q := u.Query()
	if redirectURI != "" {
		q.Set("redirect_uri", redirectURI)
	}

	if redirectURL != "" {
		q.Set("redirect_url", redirectURL)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	responseBytes, err := httpCall(config, req)
	if err != nil {
		return fmt.Errorf("error calling authurl endpoint: %s", err)
	}

	d.Set("url", string(responseBytes))
	d.SetId(fmt.Sprintf("%s-%s-%s-%d", providerName, redirectURI, redirectURL, time.Now().Unix()))

	return nil
}
