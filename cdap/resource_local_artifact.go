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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceLocalArtifact supports deploying an artifact by providing a local filepath.
// We need to use references like GCS or filepaths to avoid needing to pass and
// store the entire JAR's contents as a string.
func resourceLocalArtifact() *schema.Resource {
	return &schema.Resource{
		Create: resourceLocalArtifactCreate,
		Read:   resourceLocalArtifactRead,
		Delete: resourceLocalArtifactDelete,
		Exists: resourceLocalArtifactExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The name of the artifact.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "The name of the namespace in which this resource belongs. If not provided, the default namespace is used.",
				DefaultFunc: func() (interface{}, error) {
					return defaultNamespace, nil
				},
			},
			// Technically, we could omit the version in the API call because CDAP will infer the
			// version from the jar. However, forcing the user to specify the version makes dealing
			// with the resource easier because other API calls require it.
			"version": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The version of the artifact. Must match the version in the JAR manifest.",
			},
			"jar_binary_path": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The local path to the JAR binary for the artifact.",
			},
			"json_config_path": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The local path to the JSON config of the artifact.",
			},
		},
	}
}

type artifact struct {
	name    string
	version string
	config  *artifactConfig
	jar     []byte
}

type artifactConfig struct {
	Properties map[string]string `json:"properties"`
	Parents    []string          `json:"parents"`
}

func resourceLocalArtifactCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	a, err := loadLocalArtifact(d)
	if err != nil {
		return err
	}
	return uploadArtifact(config, d, a)
}

func uploadArtifact(config *Config, d *schema.ResourceData, a *artifact) error {
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", a.name)

	if err := uploadJar(config, addr, a); err != nil {
		return err
	}

	if err := uploadProps(config, addr, a); err != nil {
		if rbErr := deleteArtifactVersion(config, addr, a.version); rbErr != nil {
			// Rollback failed; set the resource ID so Terraform marks the resource
			// tainted and replaces it on the next apply instead of leaving an
			// untracked orphan that fails with 409.
			d.SetId(a.name)
			return fmt.Errorf("uploading properties for artifact %q version %q failed (rollback delete also failed with %v, marking resource tainted): %w", a.name, a.version, rbErr, err)
		}
		return fmt.Errorf("uploading properties for artifact %q version %q failed (uploaded JAR was rolled back): %w", a.name, a.version, err)
	}
	d.SetId(a.name)
	return nil
}

func uploadJar(config *Config, addr string, a *artifact) error {
	headers := map[string]string{
		"Artifact-Version": a.version,
	}

	if a.config != nil {
		headers["Artifact-Extends"] = strings.Join(a.config.Parents, "/")
	}

	return uploadPluginJar(config, addr, a.jar, headers)
}

func uploadProps(config *Config, artifactAddr string, a *artifact) error {
	if a.config == nil || len(a.config.Properties) == 0 {
		return nil
	}
	addr := urlJoin(artifactAddr, "/versions", a.version, "/properties")
	b, err := json.Marshal(a.config.Properties)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)
	req, err := http.NewRequest(http.MethodPut, addr, body)
	if err != nil {
		return err
	}
	if _, err := httpCall(config, req); err != nil {
		return err
	}
	return nil
}

func loadLocalArtifact(d *schema.ResourceData) (*artifact, error) {
	jar, err := ioutil.ReadFile(d.Get("jar_binary_path").(string))
	if err != nil {
		return nil, err
	}

	confb, err := ioutil.ReadFile(d.Get("json_config_path").(string))
	if err != nil {
		return nil, err
	}
	conf := new(artifactConfig)
	if err := json.Unmarshal(confb, conf); err != nil {
		return nil, err
	}

	return &artifact{
		name:    d.Get("name").(string),
		version: d.Get("version").(string),
		config:  conf,
		jar:     jar,
	}, nil
}

func resourceLocalArtifactRead(d *schema.ResourceData, m interface{}) error {
	return nil
}

func resourceLocalArtifactDelete(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", d.Get("name").(string))
	return deleteArtifactVersion(config, addr, d.Get("version").(string))
}

func deleteArtifactVersion(config *Config, artifactAddr, version string) error {
	addr := urlJoin(artifactAddr, "/versions", version)
	req, err := http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}
	if _, err := httpCall(config, req); err != nil {
		if isHTTPErrorWithCode(err, http.StatusNotFound) {
			return nil
		}
		return err
	}
	return nil
}

func resourceLocalArtifactExists(d *schema.ResourceData, m interface{}) (bool, error) {
	config := m.(*Config)
	name := d.Get("name").(string)

	namespace := d.Get("namespace").(string)
	if exists, err := namespaceExists(config, namespace); err != nil {
		return false, fmt.Errorf("failed to check for existence of namespace %q: %v", namespace, err)
	} else if !exists {
		return false, nil
	}

	return artifactExists(config, name, namespace)
}

func artifactExists(config *Config, name, namespace string) (bool, error) {
	addr := urlJoin(config.host, "/v3/namespaces", namespace, "/artifacts")

	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		return false, err
	}

	b, err := httpCall(config, req)
	if err != nil {
		return false, err
	}

	type artifact struct {
		Name string `json:"name"`
	}

	var artifacts []artifact
	if err := json.Unmarshal(b, &artifacts); err != nil {
		return false, err
	}

	log.Printf("got artifacts: %v", artifacts)

	for _, a := range artifacts {
		if a.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func uploadPluginJar(config *Config, addr string, jarBytes []byte, headers map[string]string) error {
	// An upload error is ambiguous: CDAP may have stored the JAR even though
	// the response was lost (e.g. a proxy 500 or timeout), and a replay or the
	// next apply then returns 409. Check before uploading whether the version
	// is already present so we never adopt or delete a pre-existing artifact.
	version := headers["Artifact-Version"]
	var absentBefore bool
	if version != "" {
		exists, err := artifactVersionExists(config, addr, version)
		if err != nil {
			return fmt.Errorf("checking whether artifact version %q exists before upload: %w", version, err)
		}
		absentBefore = !exists
	}

	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(jarBytes))
	if err != nil {
		return err
	}

	req.Header = map[string][]string{}
	for k, v := range headers {
		req.Header.Add(k, v)
	}

	_, err = httpCall(config, req)
	if err == nil || !absentBefore {
		return err
	}
	if config.retry.enabled && isReconciledUpload(config, addr, version, req.URL.Path, err) {
		return nil
	}
	if isAmbiguousWriteError(err) {
		// A transport error or gateway timeout (408/5xx) can occur while CDAP
		// still commits the JAR in the background. Best-effort delete prevents
		// leaving a JAR-only orphan that fails with 409 on the next apply,
		// without ever deleting a pre-existing unmanaged artifact.
		if delErr := deleteArtifactVersion(config, addr, version); delErr != nil {
			log.Printf("[WARN] best-effort cleanup DELETE for artifact version %q after failed POST (%v) also failed: %v", version, err, delErr)
		}
	}
	return err
}

func isReconciledUpload(config *Config, addr, version, reqPath string, err error) bool {
	if isHTTPErrorWithCode(err, http.StatusConflict) && wasRetried(err) {
		log.Printf("[WARN] POST %s returned 409 after a retry; treating upload of version %q as successful", reqPath, version)
		return true
	}
	// The final attempt may have been stored even though its response was a
	// transport or retryable error. The version was absent before, so if it
	// exists now this call created it.
	if isAmbiguousWriteError(err) {
		if present, checkErr := artifactVersionExists(config, addr, version); checkErr == nil && present {
			log.Printf("[WARN] POST %s failed (%v) but version %q now exists; treating upload as successful", reqPath, err, version)
			return true
		}
	}
	return false
}

func isAmbiguousWriteError(err error) bool {
	var hErr *httpError
	return (!errors.As(err, &hErr) && !errors.Is(err, context.Canceled)) || (hErr != nil && isRetryableStatusCode(hErr.code))
}

// artifactVersionExists reports whether the given version of the artifact at
// artifactAddr (…/artifacts/{name}) exists.
func artifactVersionExists(config *Config, artifactAddr, version string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, urlJoin(artifactAddr, "/versions", version), nil)
	if err != nil {
		return false, err
	}
	if _, err := httpCall(config, req); err != nil {
		if isHTTPErrorWithCode(err, http.StatusNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
