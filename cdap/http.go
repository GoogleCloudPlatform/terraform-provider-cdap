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
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	retryInitialBackoff = 1 * time.Second
	retryMaxBackoff     = 30 * time.Second
)

type httpError struct {
	code int
	body string
	// retried is true when at least one retry of the request was attempted
	// before this error was returned. Callers can use it to detect a
	// "succeeded on an earlier attempt but the response was lost" situation,
	// which typically surfaces as a 409 Conflict on replay.
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

// wasRetried reports whether err is an httpError that was returned after at least one retry.
func wasRetried(err error) bool {
	var hErr *httpError
	return errors.As(err, &hErr) && hErr.retried
}

// retryPolicy controls how httpCall handles failed requests.
type retryPolicy struct {
	// enabled turns retries on. When false, every request is attempted exactly once.
	enabled bool
	// timeout is the maximum time window, measured from the first failure,
	// during which retries are attempted.
	timeout time.Duration
	// codes is the set of HTTP status codes that are considered transient.
	codes map[int]struct{}
}

func (p retryPolicy) retryable(code int) bool {
	_, ok := p.codes[code]
	return ok
}

func urlJoin(base string, paths ...string) string {
	p := path.Join(paths...)
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), strings.TrimLeft(p, "/"))
}

// httpCall performs req, retrying it according to the provider's retry policy.
// Use it for idempotent requests (GET, DELETE, PUT-as-upsert, and POSTs that
// the server treats as idempotent). For requests whose replay would produce
// duplicate side effects, use httpCallOnce.
func httpCall(config *Config, req *http.Request) ([]byte, error) {
	if !config.retry.enabled {
		return httpCallOnce(config, req)
	}
	return httpCallWithRetry(config, req)
}

// httpCallOnce performs req exactly once, regardless of the retry policy.
func httpCallOnce(config *Config, req *http.Request) ([]byte, error) {
	prepareRequest(config, req)
	b, _, err := doRequest(config, req)
	return b, err
}

func httpCallWithRetry(config *Config, req *http.Request) ([]byte, error) {
	prepareRequest(config, req)
	policy := config.retry
	ctx := req.Context()

	var (
		deadline time.Time // zero until the first failure
		attempt  int
	)
	for {
		attempt++
		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to rewind request body for retry: %w", err)
			}
			req.Body = body
		}

		b, retryAfter, err := doRequest(config, req)
		if err == nil {
			if attempt > 1 {
				log.Printf("[INFO] %s %s succeeded on attempt %d", req.Method, req.URL.Path, attempt)
			}
			return b, nil
		}

		// Decide whether the failure is retryable at all.
		if !policy.shouldRetry(ctx, err) {
			markRetried(err, attempt > 1)
			return nil, err
		}

		if deadline.IsZero() {
			deadline = time.Now().Add(policy.timeout)
		}
		wait := backoffFor(attempt, retryAfter)
		if time.Now().Add(wait).After(deadline) {
			markRetried(err, attempt > 1)
			return nil, fmt.Errorf("giving up after %d attempt(s) within the %s retry timeout, last error: %w", attempt, policy.timeout, err)
		}

		log.Printf("[WARN] %s %s failed on attempt %d (%v), retrying in %s", req.Method, req.URL.Path, attempt, err, wait.Round(time.Millisecond))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
}

func prepareRequest(config *Config, req *http.Request) {
	if config.userAgent != "" {
		req.Header.Set("User-Agent", config.userAgent)
	}
	log.Printf("%+v", req)
}

// doRequest performs a single HTTP round trip. It returns the response body on
// 2xx, otherwise an error (an *httpError for non-2xx responses). The second
// return value carries the server's Retry-After hint, if any.
func doRequest(config *Config, req *http.Request) ([]byte, time.Duration, error) {
	resp, err := config.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return b, 0, nil
	}
	return nil, parseRetryAfter(resp.Header.Get("Retry-After")), &httpError{code: resp.StatusCode, body: string(b)}
}

// shouldRetry classifies an error returned by doRequest.
func (p retryPolicy) shouldRetry(ctx context.Context, err error) bool {
	// Never retry once the caller has given up.
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var hErr *httpError
	if errors.As(err, &hErr) {
		// 501 is permanent by definition, regardless of configuration.
		if hErr.code == http.StatusNotImplemented {
			return false
		}
		return p.retryable(hErr.code)
	}
	// Transport-level failure (connection reset, EOF, timeout, ...).
	// Treat as transient unless it is clearly a client-side configuration problem.
	msg := err.Error()
	if strings.Contains(msg, "x509:") || strings.Contains(msg, "unsupported protocol scheme") {
		return false
	}
	return true
}

func markRetried(err error, retried bool) {
	var hErr *httpError
	if errors.As(err, &hErr) {
		hErr.retried = retried
	}
}

// backoffFor returns the delay before the next attempt: an exponential backoff
// with full jitter, overridden by the server's Retry-After hint when present.
func backoffFor(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		return retryAfter
	}
	max := retryInitialBackoff << uint(attempt-1)
	if max > retryMaxBackoff || max <= 0 {
		max = retryMaxBackoff
	}
	return time.Duration(rand.Int63n(int64(max))) + retryInitialBackoff/2
}

// parseRetryAfter supports the delay-seconds form of the Retry-After header.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}
