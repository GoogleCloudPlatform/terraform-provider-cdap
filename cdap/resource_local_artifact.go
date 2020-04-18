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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
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

	if err := uploadJar(config.httpClient, addr, a); err != nil {
		return err
	}
	d.SetId(a.name)

	if err := uploadProps(config.httpClient, addr, a); err != nil {
		return err
	}
	return nil
}

func uploadJar(client *http.Client, addr string, a *artifact) error {
	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(a.jar))
	if err != nil {
		return err
	}
	req.Header = map[string][]string{}
	req.Header.Add("Artifact-Version", a.version)
	req.Header.Add("Artifact-Extends", strings.Join(a.config.Parents, "/"))
	if _, err := httpCall(client, req); err != nil {
		return err
	}
	return nil
}

func uploadProps(client *http.Client, artifactAddr string, a *artifact) error {
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
	if _, err := httpCall(client, req); err != nil {
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
	name := d.Get("name").(string)
	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", name, "/versions", d.Get("version").(string))

	req, err := http.NewRequest(http.MethodDelete, addr, nil)
	if err != nil {
		return err
	}
	_, err = httpCall(config.httpClient, req)
	return err
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

	b, err := httpCall(config.httpClient, req)
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
