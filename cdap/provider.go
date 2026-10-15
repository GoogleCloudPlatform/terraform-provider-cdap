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
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

const (
	defaultNamespace        = "default"
	defaultRetryTimeout     = 90
	defaultRetryMaxAttempts = 3
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
			"retry": &schema.Schema{
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Retry policy for transient API failures. When omitted, every API call is attempted exactly once.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"enabled": &schema.Schema{
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Whether retries are active. Defaults to false.",
						},
						"timeout_seconds": &schema.Schema{
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      defaultRetryTimeout,
							ValidateFunc: validation.IntAtLeast(1),
							Description:  "No new attempt is started once this many seconds have elapsed since the first attempt began. A single in-flight request is bounded by the HTTP client timeout, not this value. Defaults to 90 (90s) when enabled is set to true.",
						},
						"max_attempts": &schema.Schema{
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      defaultRetryMaxAttempts,
							ValidateFunc: validation.IntAtLeast(0),
							Description:  "Maximum number of attempts per API call, including the first. Defaults to 3 when enabled is set to true; 0 means attempts are limited only by timeout_seconds. Whichever limit is reached first stops retrying.",
						},
					},
				},
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
	retry         retryPolicy
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

		return &Config{
			host:          d.Get("host").(string),
			httpClient:    httpClient,
			storageClient: storageClient,
			userAgent:     userAgent,
			retry:         expandRetryPolicy(d.Get("retry").([]interface{})),
		}, nil
	}
}

// expandRetryPolicy converts the provider's retry block into a retryPolicy.
// An absent block yields a disabled policy, preserving the historical
// single-attempt behaviour.
func expandRetryPolicy(raw []interface{}) retryPolicy {
	if len(raw) == 0 || raw[0] == nil {
		return retryPolicy{}
	}
	block := raw[0].(map[string]interface{})

	policy := retryPolicy{
		enabled:     block["enabled"].(bool),
		timeout:     time.Duration(block["timeout_seconds"].(int)) * time.Second,
		maxAttempts: block["max_attempts"].(int),
	}
	log.Printf("[INFO] cdap provider retry policy: enabled=%t timeout=%s max_attempts=%d", policy.enabled, policy.timeout, policy.maxAttempts)
	return policy
}

func isGoogleAPIErrorWithCode(err error, codes ...int) bool {
	gErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	return slices.Contains(codes, gErr.Code)
}
