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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"regexp"

	"cloud.google.com/go/storage"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

var bucketPathRE = regexp.MustCompile(`^gs://(.+)/(.+)$`)

// resourceGCSArtifact supports deploying an artifact by providing a GCS path.
// We need to use references like GCS or filepaths to avoid needing to pass and
// store the entire JAR's contents as a string.
func resourceGCSArtifact() *schema.Resource {
	return &schema.Resource{
		Create: resourceGCSArtifactCreate,
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
				Description: "The GCS path to the JAR binary for the artifact.",
			},
			"json_config_path": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The GCS path to the JSON config of the artifact.",
			},
		},
	}
}

func resourceGCSArtifactCreate(d *schema.ResourceData, m interface{}) error {
	ctx := context.Background()
	config := m.(*Config)

	a, err := loadGCSArtifact(ctx, d, config.storageClient)
	if err != nil {
		return err
	}
	return uploadArtifact(config, d, a)
}

func loadGCSArtifact(ctx context.Context, d *schema.ResourceData, storageClient *storage.Client) (*artifact, error) {
	jar, err := readObject(ctx, storageClient, d.Get("jar_binary_path").(string))
	if err != nil {
		return nil, err
	}

	confb, err := readObject(ctx, storageClient, d.Get("json_config_path").(string))
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
		jar:     jar,
		config:  conf,
	}, nil
}

func readObject(ctx context.Context, storageClient *storage.Client, path string) ([]byte, error) {
	// matches is in the form [matched substring, bucket name, object name].
	matches := bucketPathRE.FindStringSubmatch(path)
	if len(matches) != 3 {
		return nil, fmt.Errorf("unexpected bucket path: got %q submatches, want 3", len(matches))
	}
	bucketName, objectPath := matches[1], matches[2]
	obj := storageClient.Bucket(bucketName).Object(objectPath)
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(r)
}
