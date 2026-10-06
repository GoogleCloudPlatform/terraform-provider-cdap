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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

type httpError struct {
	code int
	body string
	// retried is true when the request was attempted more than once before
	// this error was returned. A 409 Conflict seen only after a retry usually
	// means an earlier attempt succeeded but its response was lost.
	retried bool
}

func (e *httpError) Error() string {
	return fmt.Sprintf("%v: %v", e.code, e.body)
}

// isHTTPErrorWithCode reports whether err is an httpError with one of the given codes.
func isHTTPErrorWithCode(err error, codes ...int) bool {
	var hErr *httpError
	if !errors.As(err, &hErr) {
		return false
	}
	for _, c := range codes {
		if hErr.code == c {
			return true
		}
	}
	return false
}

// wasRetried reports whether err is an httpError returned after at least one retry.
func wasRetried(err error) bool {
	var hErr *httpError
	return errors.As(err, &hErr) && hErr.retried
}

// retryPolicy controls how httpCall handles failed requests.
type retryPolicy struct {
	enabled bool
	timeout time.Duration
	codes   map[int]struct{} // HTTP status codes considered transient
}

func urlJoin(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}

// httpCall performs req, retrying it according to the provider's retry policy.
// Use httpCallOnce for requests whose replay would cause duplicate side effects.
func httpCall(config *Config, req *http.Request) ([]byte, error) {
	if !config.retry.enabled {
		return httpCallOnce(config, req)
	}

	var respBytes []byte
	attempt := 0
	err := resource.RetryContext(req.Context(), config.retry.timeout, func() *resource.RetryError {
		attempt++
		if req.GetBody != nil {
			req.Body, _ = req.GetBody()
		}

		b, err := doRequest(config, req)
		if err == nil {
			respBytes = b
			return nil
		}

		var hErr *httpError
		if errors.As(err, &hErr) {
			hErr.retried = attempt > 1
			// 501 is permanent by definition; everything else follows the allowlist.
			if _, ok := config.retry.codes[hErr.code]; !ok || hErr.code == http.StatusNotImplemented {
				return resource.NonRetryableError(err)
			}
		}
		log.Printf("[WARN] %s %s failed on attempt %d, retrying: %v", req.Method, req.URL.Path, attempt, err)
		return resource.RetryableError(err)
	})
	return respBytes, err
}

// httpCallOnce performs req exactly once, regardless of the retry policy.
func httpCallOnce(config *Config, req *http.Request) ([]byte, error) {
	return doRequest(config, req)
}

// doRequest performs a single HTTP round trip and returns the response body
// on 2xx, or an *httpError for any other status.
func doRequest(config *Config, req *http.Request) ([]byte, error) {
	if config.userAgent != "" {
		req.Header.Set("User-Agent", config.userAgent)
	}
	log.Printf("[DEBUG] %+v", req)

	resp, err := config.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpError{code: resp.StatusCode, body: string(b)}
	}
	return b, nil
}
