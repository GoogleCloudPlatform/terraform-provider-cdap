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
		Read:   resourceArtifactRead,
		Delete: resourceArtifactDelete,
		Exists: resourceArtifactExists,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
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
			"bucket_path": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Path to GCS bucket object containing the spec, JAR and JSON.",
			},
			"version": {
				Type:        schema.TypeString,
				Computed:    true,
				ForceNew:    true,
				Description: "The version of the artifact.",
			},
		},
	}
}

type artifactSpec struct {
	Actions []*action `json:"actions"`
}
type action struct {
	Type string `json:"type"`
	Args []*struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"arguments"`
}

func resourceGCSArtifactCreate(d *schema.ResourceData, m interface{}) error {
	ctx := context.Background()
	config := m.(*Config)

	// matches is in the form [matched substring, bucket name, object name].
	matches := bucketPathRE.FindStringSubmatch(d.Get("bucket_path").(string))
	if len(matches) != 3 {
		return fmt.Errorf("unexpected bucket path: got %q submatches, want 3", len(matches))
	}

	bucketName, objectPath := matches[1], matches[2]
	bucket := config.storageClient.Bucket(bucketName)
	data, err := loadDataFromGCS(ctx, bucket, objectPath)
	if err != nil {
		return err
	}

	addr := urlJoin(config.host, "/v3/namespaces", d.Get("namespace").(string), "/artifacts", data.name)

	if err := uploadJar(config.client, addr, data); err != nil {
		return err
	}
	if err := uploadProps(config.client, addr, data); err != nil {
		return err
	}

	d.Set("name", data.name)
	d.Set("version", data.version)
	d.SetId(data.name)
	return nil
}

func loadDataFromGCS(ctx context.Context, bucket *storage.BucketHandle, objectPath string) (*artifactData, error) {
	specObj := bucket.Object(urlJoin(objectPath, "spec.json"))
	b, err := readObject(ctx, specObj)
	if err != nil {
		return nil, err
	}

	spec := new(artifactSpec)
	if err := json.Unmarshal(b, spec); err != nil {
		return nil, err
	}

	if len(spec.Actions) != 1 {
		return nil, fmt.Errorf("only 1 action is currently supported, got %v", len(spec.Actions))
	}
	action := spec.Actions[0]
	if got, want := action.Type, "one_step_deploy_plugin"; got != want {
		return nil, fmt.Errorf("only action of type %q is currently supported, got %q", want, got)
	}
	return loadDataFromAction(ctx, bucket, objectPath, action)
}

func loadDataFromAction(ctx context.Context, bucket *storage.BucketHandle, objectPath string, action *action) (*artifactData, error) {
	wantArgs := map[string]bool{
		"name":    true,
		"version": true,
		"config":  true,
		"jar":     true,
	}

	data := &artifactData{}
	for _, arg := range action.Args {
		switch arg.Name {
		case "name":
			delete(wantArgs, "name")
			data.name = arg.Value
		case "version":
			delete(wantArgs, "version")
			data.version = arg.Value
		case "config":
			delete(wantArgs, "config")
			confObj := bucket.Object(urlJoin(objectPath, arg.Value))
			b, err := readObject(ctx, confObj)
			if err != nil {
				return nil, err
			}
			data.config = new(artifactConfig)
			if err := json.Unmarshal(b, data.config); err != nil {
				return nil, err
			}
		case "jar":
			delete(wantArgs, "jar")
			jarObj := bucket.Object(urlJoin(objectPath, arg.Value))
			b, err := readObject(ctx, jarObj)
			if err != nil {
				return nil, err
			}
			data.jar = b
		}
	}

	if len(wantArgs) != 0 {
		return nil, fmt.Errorf("failed to find artifact fields %v", wantArgs)
	}
	return data, nil
}

func readObject(ctx context.Context, obj *storage.ObjectHandle) ([]byte, error) {
	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(r)
}
