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
	"fmt"
	"log"
	"net/http"
	"slices"
	"time"

	"cloud.google.com/go/storage"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	defaultNamespace    = "default"
	defaultRetryTimeout = 90
)

// Provider returns a terraform.ResourceProvider.
func Provider(version string) *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The address of the CDAP instance.",
			},
			"token": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The OAuth token to use for all http calls to the instance.",
			},
			"retry_timeout": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     defaultRetryTimeout,
				Description: "The maximum duration in seconds to retry an API call that fails with a transient error (any connection failure or HTTP 4xx/5xx except 401 and 404). Defaults to 90.",
			},
		},
		ConfigureFunc: configureProvider(version),
		ResourcesMap: map[string]*schema.Resource{
			"cdap_application":           resourceApplication(),
			"cdap_streaming_program_run": resourceStreamingProgramRun(),
			"cdap_gcs_artifact":          resourceGCSArtifact(),
			"cdap_gcs_jdbc_driver":       resourceGCSJDBCDriver(),
			"cdap_local_artifact":        resourceLocalArtifact(),
			"cdap_local_jdbc_driver":     resourceLocalJDBCDriver(),
			"cdap_namespace":             resourceNamespace(),
			"cdap_namespace_preferences": resourceNamespacePreferences(),
			"cdap_profile":               resourceProfile(),
			"cdap_oauth_provider":        resourceOAuthProvider(),
			"cdap_oauth_credential":      resourceOAuthCredential(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"cdap_oauth_url":                   dataSourceOAuthURL(),
			"cdap_oauth_credential":            dataSourceOAuthCredential(),
			"cdap_oauth_credential_validation": dataSourceOAuthCredentialValidation(),
		},
	}
}

// Config provides service configuration for service clients.
type Config struct {
	host          string
	httpClient    *http.Client
	storageClient *storage.Client
	userAgent     string
	retryTimeout  time.Duration
}

func configureProvider(version string) schema.ConfigureFunc {
	return func(d *schema.ResourceData) (interface{}, error) {
		ctx := context.Background()

		httpClient := &http.Client{}
		if token, ok := d.GetOk("token"); ok {
			httpClient = oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{
				AccessToken: token.(string),
				TokenType:   "Bearer",
			}))
		}
		httpClient.Timeout = 30 * time.Minute

		opts := []option.ClientOption{option.WithScopes(storage.ScopeReadOnly)}
		storageClient, err := storage.NewClient(ctx, opts...)
		if err != nil {
			log.Printf("Authenticated client failed : %v, falling back to unauthenticated...", err)
			if isGoogleAPIErrorWithCode(err, http.StatusForbidden, http.StatusUnauthorized) {
				opts = append(opts, option.WithoutAuthentication())
				storageClient, err = storage.NewClient(ctx, opts...)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to create storage client: %w", err)
			}
		}

		userAgent := fmt.Sprintf("terraform-provider-cdap/%s", version)
		retryTimeout := time.Duration(d.Get("retry_timeout").(int)) * time.Second

		return &Config{
			host:          d.Get("host").(string),
			httpClient:    httpClient,
			storageClient: storageClient,
			userAgent:     userAgent,
			retryTimeout:  retryTimeout,
		}, nil
	}
}

func isGoogleAPIErrorWithCode(err error, codes ...int) bool {
	gErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	return slices.Contains(codes, gErr.Code)
}
