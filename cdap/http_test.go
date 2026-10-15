// Copyright 2026 Google LLC
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
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPCall(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		body         []byte
		retry        retryPolicy
		once         bool
		responses    []int
		wantAttempts int32
		wantErr      bool
	}{
		{
			name:         "retry disabled fails on first 500",
			method:       http.MethodGet,
			retry:        retryPolicy{enabled: false},
			responses:    []int{http.StatusInternalServerError, http.StatusOK},
			wantAttempts: 1,
			wantErr:      true,
		},
		{
			name:         "503 twice then 200 succeeds with body rewind",
			method:       http.MethodPut,
			body:         []byte(`{"key":"val"}`),
			retry:        retryPolicy{enabled: true, timeout: 10 * time.Second, maxAttempts: 3},
			responses:    []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK},
			wantAttempts: 3,
			wantErr:      false,
		},
		{
			name:         "429 then 200 succeeds",
			method:       http.MethodGet,
			retry:        retryPolicy{enabled: true, timeout: 10 * time.Second, maxAttempts: 3},
			responses:    []int{http.StatusTooManyRequests, http.StatusOK},
			wantAttempts: 2,
			wantErr:      false,
		},
		{
			name:         "408 then 200 succeeds",
			method:       http.MethodPost,
			body:         []byte("jar-bytes"),
			retry:        retryPolicy{enabled: true, timeout: 10 * time.Second, maxAttempts: 3},
			responses:    []int{http.StatusRequestTimeout, http.StatusOK},
			wantAttempts: 2,
			wantErr:      false,
		},
		{
			name:         "401 non-retryable fails immediately",
			method:       http.MethodGet,
			retry:        retryPolicy{enabled: true, timeout: 10 * time.Second, maxAttempts: 3},
			responses:    []int{http.StatusUnauthorized, http.StatusOK},
			wantAttempts: 1,
			wantErr:      true,
		},
		{
			name:         "max_attempts caps retries",
			method:       http.MethodGet,
			retry:        retryPolicy{enabled: true, timeout: 30 * time.Second, maxAttempts: 2},
			responses:    []int{http.StatusServiceUnavailable, http.StatusServiceUnavailable, http.StatusOK},
			wantAttempts: 2,
			wantErr:      true,
		},
		{
			name:         "timeout stops starting new attempts",
			method:       http.MethodGet,
			retry:        retryPolicy{enabled: true, timeout: 300 * time.Millisecond, maxAttempts: 0},
			responses:    []int{http.StatusBadGateway, http.StatusBadGateway},
			wantAttempts: 1,
			wantErr:      true,
		},
		{
			name:         "httpCallOnce never retries",
			method:       http.MethodPost,
			body:         []byte("{}"),
			retry:        retryPolicy{enabled: true, timeout: 10 * time.Second, maxAttempts: 3},
			once:         true,
			responses:    []int{http.StatusServiceUnavailable, http.StatusOK},
			wantAttempts: 1,
			wantErr:      true,
		},
		{
			name:         "DELETE 502 then 404 reconciled as success",
			method:       http.MethodDelete,
			retry:        retryPolicy{enabled: true, timeout: 10 * time.Second, maxAttempts: 3},
			responses:    []int{http.StatusBadGateway, http.StatusNotFound},
			wantAttempts: 2,
			wantErr:      false,
		},
		{
			name:         "DELETE 404 on first attempt returns error",
			method:       http.MethodDelete,
			retry:        retryPolicy{enabled: true, timeout: 10 * time.Second, maxAttempts: 3},
			responses:    []int{http.StatusNotFound},
			wantAttempts: 1,
			wantErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var attempts int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				idx := int(atomic.AddInt32(&attempts, 1)) - 1
				if tc.body != nil {
					buf := new(bytes.Buffer)
					_, _ = buf.ReadFrom(r.Body)
					if !bytes.Equal(buf.Bytes(), tc.body) {
						t.Errorf("attempt %d body = %q, want %q", idx+1, buf.Bytes(), tc.body)
					}
				}
				if idx >= len(tc.responses) {
					idx = len(tc.responses) - 1
				}
				w.WriteHeader(tc.responses[idx])
			}))
			defer srv.Close()

			cfg := &Config{
				host:       srv.URL,
				httpClient: srv.Client(),
				retry:      tc.retry,
			}

			var req *http.Request
			var err error
			if tc.body != nil {
				req, err = http.NewRequest(tc.method, srv.URL+"/v3/test", bytes.NewReader(tc.body))
			} else {
				req, err = http.NewRequest(tc.method, srv.URL+"/v3/test", nil)
			}
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}

			if tc.once {
				_, err = httpCallOnce(cfg, req)
			} else {
				_, err = httpCall(cfg, req)
			}
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got := atomic.LoadInt32(&attempts); got != tc.wantAttempts {
				t.Fatalf("attempts = %d, want %d", got, tc.wantAttempts)
			}
		})
	}
}

func TestUploadPluginJarReconcile(t *testing.T) {
	tests := []struct {
		name          string
		maxAttempts   int
		getResponses  []int
		postResponses []int
		wantGETs      int32
		wantPOSTs     int32
		wantErr       bool
	}{
		{
			name:          "absent before, POST 503 then 409 reconciled as success",
			maxAttempts:   3,
			getResponses:  []int{http.StatusNotFound},
			postResponses: []int{http.StatusServiceUnavailable, http.StatusConflict},
			wantGETs:      1,
			wantPOSTs:     2,
			wantErr:       false,
		},
		{
			name:          "absent before, POST 409 on first attempt fails",
			maxAttempts:   3,
			getResponses:  []int{http.StatusNotFound},
			postResponses: []int{http.StatusConflict},
			wantGETs:      1,
			wantPOSTs:     1,
			wantErr:       true,
		},
		{
			name:          "pre-existed before, POST 503 then 409 not adopted",
			maxAttempts:   3,
			getResponses:  []int{http.StatusOK},
			postResponses: []int{http.StatusServiceUnavailable, http.StatusConflict},
			wantGETs:      1,
			wantPOSTs:     2,
			wantErr:       true,
		},
		{
			name:          "absent before, final attempt 502 but version now exists reconciled",
			maxAttempts:   1,
			getResponses:  []int{http.StatusNotFound, http.StatusOK},
			postResponses: []int{http.StatusBadGateway},
			wantGETs:      2,
			wantPOSTs:     1,
			wantErr:       false,
		},
		{
			name:          "absent before, POST 400 fails without post-check GET",
			maxAttempts:   3,
			getResponses:  []int{http.StatusNotFound, http.StatusOK},
			postResponses: []int{http.StatusBadRequest},
			wantGETs:      1,
			wantPOSTs:     1,
			wantErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gets, posts int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					idx := int(atomic.AddInt32(&gets, 1)) - 1
					if idx >= len(tc.getResponses) {
						idx = len(tc.getResponses) - 1
					}
					w.WriteHeader(tc.getResponses[idx])
				case http.MethodPost:
					idx := int(atomic.AddInt32(&posts, 1)) - 1
					if idx >= len(tc.postResponses) {
						idx = len(tc.postResponses) - 1
					}
					w.WriteHeader(tc.postResponses[idx])
				}
			}))
			defer srv.Close()

			cfg := &Config{
				host:       srv.URL,
				httpClient: srv.Client(),
				retry: retryPolicy{
					enabled:     true,
					timeout:     10 * time.Second,
					maxAttempts: tc.maxAttempts,
				},
			}

			addr := urlJoin(cfg.host, "/v3/namespaces/default/artifacts/my-plugin")
			err := uploadPluginJar(cfg, addr, []byte("jar"), map[string]string{"Artifact-Version": "1.0.0"})
			if (err != nil) != tc.wantErr {
				t.Fatalf("uploadPluginJar err = %v, wantErr = %v", err, tc.wantErr)
			}
			if got := atomic.LoadInt32(&gets); got != tc.wantGETs {
				t.Fatalf("GET count = %d, want %d", got, tc.wantGETs)
			}
			if got := atomic.LoadInt32(&posts); got != tc.wantPOSTs {
				t.Fatalf("POST count = %d, want %d", got, tc.wantPOSTs)
			}
		})
	}
}

func TestExpandRetryPolicy(t *testing.T) {
	tests := []struct {
		name string
		raw  []interface{}
		want retryPolicy
	}{
		{
			name: "empty slice is disabled",
			raw:  nil,
			want: retryPolicy{enabled: false},
		},
		{
			name: "enabled false stays disabled",
			raw: []interface{}{
				map[string]interface{}{
					"enabled":         false,
					"timeout_seconds": 90,
					"max_attempts":    3,
				},
			},
			want: retryPolicy{
				enabled:     false,
				timeout:     90 * time.Second,
				maxAttempts: 3,
			},
		},
		{
			name: "enabled true populates timeout and maxAttempts",
			raw: []interface{}{
				map[string]interface{}{
					"enabled":         true,
					"timeout_seconds": 120,
					"max_attempts":    5,
				},
			},
			want: retryPolicy{
				enabled:     true,
				timeout:     120 * time.Second,
				maxAttempts: 5,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandRetryPolicy(tc.raw); got != tc.want {
				t.Errorf("expandRetryPolicy(%v) = %+v, want %+v", tc.raw, got, tc.want)
			}
		})
	}
}
