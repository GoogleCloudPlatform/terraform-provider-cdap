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

// Generate_docs generates documentation for a provider schema.
package main

import (
	"flag"
	"log"

	"terraform-provider-cdap/cdap"
)

var (
	tmplDir   = flag.String("template_dir", "", "Directory containing template files")
	outputDir = flag.String("output_dir", "", "Directory to write generated docs")
)

func main() {
	flag.Parse()
	if *tmplDir == "" {
		log.Fatal("--template_dir must be set")
	}
	if *outputDir == "" {
		log.Fatal("--output_dir must be set")
	}
	if err := generate(cdap.Provider(), *tmplDir, *outputDir); err != nil {
		log.Fatalf("failed to generate docs: %v", err)
	}
}
