/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package hyperstack

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxRetries      int           // Maximum number of retries (default: 3)
	BaseDelay       time.Duration // Base delay between retries (default: 100ms)
	MaxDelay        time.Duration // Maximum delay between retries (default: 5s)
	RetryableErrors []int         // HTTP status codes that should be retried (default: 5xx, 429)
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      3,
		BaseDelay:       100 * time.Millisecond,
		MaxDelay:        5 * time.Second,
		RetryableErrors: []int{429, 500, 502, 503, 504},
	}
}

// isRetryableError checks if the given HTTP status code should be retried
func (rc *RetryConfig) isRetryableError(statusCode int) bool {
	for _, code := range rc.RetryableErrors {
		if statusCode == code {
			return true
		}
	}
	return false
}

// calculateDelay calculates the delay for the given attempt using exponential backoff
func (rc *RetryConfig) calculateDelay(attempt int) time.Duration {
	delay := time.Duration(float64(rc.BaseDelay) * math.Pow(2, float64(attempt)))
	if delay > rc.MaxDelay {
		delay = rc.MaxDelay
	}
	jitter := time.Duration(float64(delay) * (0.5 + rand.Float64()) * 0.5)
	return jitter
}

// RetryableHTTPClient wraps an http.Client with retry logic
type RetryableHTTPClient struct {
	client      *http.Client
	retryConfig *RetryConfig
}

// NewRetryableHTTPClient creates a new retryable HTTP client
func NewRetryableHTTPClient(client *http.Client, retryConfig *RetryConfig) *RetryableHTTPClient {
	if retryConfig == nil {
		retryConfig = DefaultRetryConfig()
	}
	return &RetryableHTTPClient{
		client:      client,
		retryConfig: retryConfig,
	}
}

// HTTP request with retry logic
func (r *RetryableHTTPClient) Do(req *http.Request) (*http.Response, error) {
	var lastErr error
	var lastResp *http.Response

	for attempt := 0; attempt <= r.retryConfig.MaxRetries; attempt++ {
		// Check if context is cancelled before making the request
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		default:
		}

		resp, err := r.client.Do(req)
		if err != nil {
			lastErr = err
			// Network errors are always retryable
			if attempt < r.retryConfig.MaxRetries {
				delay := r.retryConfig.calculateDelay(attempt)
				time.Sleep(delay)
				continue
			}
			return nil, err
		}

		// Check if the response status code is retryable
		if r.retryConfig.isRetryableError(resp.StatusCode) {
			lastResp = resp
			if attempt < r.retryConfig.MaxRetries {
				resp.Body.Close()
				delay := r.retryConfig.calculateDelay(attempt)
				time.Sleep(delay)
				continue
			}
			return resp, nil
		}

		// Success or non-retryable error
		return resp, nil
	}

	// If we get here, we've exhausted all retries
	if lastResp != nil {
		return lastResp, nil
	}
	return nil, fmt.Errorf("max retries exceeded: %v", lastErr)
}

// TimeoutConfig holds timeout configuration for different operation types
type TimeoutConfig struct {
	ReadTimeout  time.Duration // Timeout for read operations (default: 3s)
	WriteTimeout time.Duration // Timeout for write operations (default: 15s)
}

// DefaultTimeoutConfig returns sensible default timeout configuration
func DefaultTimeoutConfig() *TimeoutConfig {
	return &TimeoutConfig{
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
}

// WithTimeout creates a new context with the appropriate timeout based on the HTTP method
func WithTimeout(ctx context.Context, method string, timeoutConfig *TimeoutConfig) (context.Context, context.CancelFunc) {
	if timeoutConfig == nil {
		timeoutConfig = DefaultTimeoutConfig()
	}

	var timeout time.Duration
	switch method {
	case "GET", "HEAD", "OPTIONS":
		timeout = timeoutConfig.ReadTimeout
	default:
		timeout = timeoutConfig.WriteTimeout
	}

	return context.WithTimeout(ctx, timeout)
}
