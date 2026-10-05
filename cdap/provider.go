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
	defaultNamespace    = "default"
	defaultRetryTimeout = 90
)

// defaultRetryErrorCodes are the HTTP status codes retried when a retry block
// is configured without an explicit error_codes list.
var defaultRetryErrorCodes = []int{
	http.StatusTooManyRequests,
	http.StatusInternalServerError,
	http.StatusBadGateway,
	http.StatusServiceUnavailable,
	http.StatusGatewayTimeout,
}

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
							Default:     true,
							Description: "Whether retries are active. Defaults to true when the retry block is present.",
						},
						"timeout": &schema.Schema{
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      defaultRetryTimeout,
							ValidateFunc: validation.IntAtLeast(1),
							Description:  "Maximum time in seconds, including the first attempt, that a failed API call is retried for. Defaults to 90.",
						},
						"error_codes": &schema.Schema{
							Type:        schema.TypeList,
							Optional:    true,
							Description: "HTTP status codes treated as transient and retried. Connection errors are always retried. Defaults to [429, 500, 502, 503, 504].",
							Elem: &schema.Schema{
								Type: schema.TypeInt,
								ValidateFunc: validation.All(
									validation.IntBetween(400, 599),
									validation.IntNotInSlice([]int{http.StatusNotImplemented}),
								),
							},
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
	policy := retryPolicy{codes: map[int]struct{}{}}
	if len(raw) == 0 || raw[0] == nil {
		return policy
	}
	block := raw[0].(map[string]interface{})

	policy.enabled = block["enabled"].(bool)
	policy.timeout = time.Duration(block["timeout"].(int)) * time.Second

	codes := defaultRetryErrorCodes
	if l, ok := block["error_codes"].([]interface{}); ok && len(l) > 0 {
		codes = make([]int, 0, len(l))
		for _, c := range l {
			codes = append(codes, c.(int))
		}
	}
	for _, c := range codes {
		policy.codes[c] = struct{}{}
	}

	log.Printf("[INFO] cdap provider retry policy: enabled=%t timeout=%s error_codes=%v", policy.enabled, policy.timeout, codes)
	return policy
}

func isGoogleAPIErrorWithCode(err error, codes ...int) bool {
	gErr, ok := err.(*googleapi.Error)
	if !ok {
		return false
	}
	return slices.Contains(codes, gErr.Code)
}
