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
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func resourceLocalArtifact() *schema.Resource {
	return &schema.Resource{
		Create: resourceLocalArtifactCreate,
		Read:   resourceArtifactRead,
		Delete: resourceArtifactDelete,
		Exists: resourceArtifactExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				DefaultFunc: func() (interface{}, error) {
					return defaultNamespace, nil
				},
			},
			// Technically, we could omit the version in the API call because CDAP will infer the
			// version from the jar. However, forcing the user to specify the version makes dealing
			// with the resource easier because other API calls require it.
			"version": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"jar_binary_path": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"json_configuration_path": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
		},
	}
}

func resourceLocalArtifactCreate(d *schema.ResourceData, m interface{}) error {
	config := m.(*Config)
	ad, err := initArtifactData(config, d)
	if err != nil {
		return err
	}
	defer ad.Close()
	if err := uploadJar(config.client, ad); err != nil {
		return err
	}
	if err := uploadProps(config.client, ad); err != nil {
		return err
	}
	d.SetId(ad.name)
	return nil
}

func uploadJar(client *http.Client, d *artifactData) error {
	req, err := http.NewRequest(http.MethodPost, d.resourceURL, d.jar)
	if err != nil {
		return err
	}
	req.Header = map[string][]string{}
	req.Header.Add("Artifact-Version", d.version)
	req.Header.Add("Artifact-Extends", strings.Join(d.config.Parents, "/"))
	if _, err := httpCall(client, req); err != nil {
		return err
	}
	return nil
}

func uploadProps(client *http.Client, d *artifactData) error {
	addr := urlJoin(d.resourceURL, "/versions", d.version, "/properties")
	b, err := json.Marshal(d.config.Properties)
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

type artifactData struct {
	name        string
	version     string
	resourceURL string
	config      *artifactConfig
	jar         *os.File
}

func initArtifactData(c *Config, d *schema.ResourceData) (*artifactData, error) {
	ad := artifactData{}
	ad.name = d.Get("name").(string)
	ad.resourceURL = urlJoin(c.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", ad.name)
	jar, err := os.Open(d.Get("jar_binary_path").(string))
	if err != nil {
		return nil, err
	}
	ad.jar = jar
	ac, err := readArtifactConfig(d.Get("json_configuration_path").(string))
	if err != nil {
		return nil, err
	}
	ad.config = ac
	ad.version = d.Get("version").(string)
	return &ad, nil
}

func (d artifactData) Close() error {
	return d.jar.Close()
}

type artifactConfig struct {
	Properties map[string]string `json:"properties"`
	Parents    []string          `json:"parents"`
}

func readArtifactConfig(fileName string) (*artifactConfig, error) {
	f, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	var c artifactConfig
	if err = json.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
