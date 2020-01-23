package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func generate(provider *schema.Provider, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	var buf bytes.Buffer
	args := templateArgs{Title: "CDAP Provider", Schema: provider.Schema}
	if err := markdownTemplate.Execute(&buf, args); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(outputDir, "provider.md"), buf.Bytes(), 0644); err != nil {
		return err
	}

	for name, res := range provider.ResourcesMap {
		var buf bytes.Buffer
		args := templateArgs{Title: name, Schema: res.Schema}
		if err := markdownTemplate.Execute(&buf, args); err != nil {
			return err
		}
		p := filepath.Join(outputDir, "r", fmt.Sprintf("%s.md", name))
		if err := ioutil.WriteFile(p, buf.Bytes(), 0644); err != nil {
			return err
		}
	}
	return nil
}
