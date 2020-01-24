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
	"context"
	"net/http"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"golang.org/x/oauth2"
)

const defaultNamespace = "default"

// Provider returns a terraform.ResourceProvider.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
			"token": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
		},
		ConfigureFunc: configureProvider,
		ResourcesMap: map[string]*schema.Resource{
			"cdap_application":           resourceApplication(),
			"cdap_artifact":              resourceArtifact(),
			"cdap_artifact_property":     resourceArtifactProperty(),
			"cdap_namespace":             resourceNamespace(),
			"cdap_namespace_preferences": resourceNamespacePreferences(),
			"cdap_profile":               resourceProfile(),
		},
	}
}

// Config provides service configuration for service clients.
type Config struct {
	host   string
	client *http.Client
}

func configureProvider(d *schema.ResourceData) (interface{}, error) {
	ctx := context.Background()
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: d.Get("token").(string),
		TokenType:   "Bearer",
	}))
	client.Timeout = 30 * time.Minute

	return &Config{
		host:   d.Get("host").(string),
		client: client,
	}, nil
}
