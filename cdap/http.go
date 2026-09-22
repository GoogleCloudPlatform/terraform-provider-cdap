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
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

type httpError struct {
	code int
	body string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%v: %v", e.code, e.body)
}

func urlJoin(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}

func httpCall(config *Config, req *http.Request) ([]byte, error) {
	if config.userAgent != "" {
		req.Header.Set("User-Agent", config.userAgent)
	}

	log.Printf("%+v", req)

	var respBytes []byte
	err := resource.RetryContext(req.Context(), config.retryTimeout, func() *resource.RetryError {
		if req.GetBody != nil {
			req.Body, _ = req.GetBody()
		}

		resp, err := config.httpClient.Do(req)
		if err != nil {
			return resource.RetryableError(err)
		}
		defer resp.Body.Close()

		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return resource.RetryableError(err)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			respBytes = b
			return nil
		}

		httpErr := &httpError{code: resp.StatusCode, body: string(b)}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
			return resource.NonRetryableError(httpErr)
		}
		log.Printf("[WARN] %s %s failed with %d, retrying: %v", req.Method, req.URL.Path, resp.StatusCode, httpErr)
		return resource.RetryableError(httpErr)
	})
	return respBytes, err
}
