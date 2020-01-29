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

// Package cdap provides a Terraform provider to manage CDAP APIs.
package cdap

import (
	"context"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

const defaultNamespace = "default"

// Provider returns a terraform.ResourceProvider.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				Description: "The address of the CDAP instance.",
			},
			"token": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				Description: "The OAuth token to use for all http calls to the instance.",
			},
		},
		ConfigureFunc: configureProvider,
		ResourcesMap: map[string]*schema.Resource{
			"cdap_application":           resourceApplication(),
			"cdap_artifact":              resourceArtifact(),
			"cdap_gcs_artifact":          resourceGCSArtifact(),
			"cdap_local_artifact":        resourceLocalArtifact(),
			"cdap_artifact_property":     resourceArtifactProperty(),
			"cdap_namespace":             resourceNamespace(),
			"cdap_namespace_preferences": resourceNamespacePreferences(),
			"cdap_profile":               resourceProfile(),
		},
	}
}

// Config provides service configuration for service clients.
type Config struct {
	host          string
	client        *http.Client // TODO: rename to httpClient
	storageClient *storage.Client
}

func configureProvider(d *schema.ResourceData) (interface{}, error) {
	ctx := context.Background()
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: d.Get("token").(string),
		TokenType:   "Bearer",
	}))
	client.Timeout = 30 * time.Minute

	storageClient, err := storage.NewClient(ctx, option.WithScopes(storage.ScopeReadOnly))
	if err != nil {
		return nil, err
	}

	return &Config{
		host:          d.Get("host").(string),
		client:        client,
		storageClient: storageClient,
	}, nil
}
