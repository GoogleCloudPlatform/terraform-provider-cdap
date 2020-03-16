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
	"math"
	"net/http"
	"path"
	"strings"
	"time"
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

func httpCall(client *http.Client, req *http.Request) ([]byte, error) {
	log.Printf("%+v", req)

	b, err := doHTTPCall(client, req)

	// CDAP REST intermittently returns 500, 504, etc. internal errors, we will retry on 5xxs once.
	var e *httpError
	// poor man's 3x EBO.
	for i := 0; i < 3; i++ {
		if errors.As(err, &e) && e.code >= 500 && e.code < 600 {
			log.Printf("retrying on intermittent 5xx error in %v seconds", s)
			// Have to explicitly cast if int var: https://play.golang.org/d0ZFLSVoAw
			time.Sleep(time.Duration(math.Pow(2, i)) * time.Second)
			b, err = doHTTPCall(client, req)
			if err != nil {
				break
			}
		}
	}
	return b, err
}

func doHTTPCall(client *http.Client, req *http.Request) ([]byte, error) {
	resp, err := client.Do(req)
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
