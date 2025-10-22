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
	"net/http"
	"testing"
	"time"
)

// MockHTTPClient for testing
type MockHTTPClient struct {
	responses []*http.Response
	errors    []error
	callCount int
}

func (m *MockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.callCount >= len(m.responses) && m.callCount >= len(m.errors) {
		return nil, fmt.Errorf("no more mock responses")
	}

	m.callCount++

	if m.callCount <= len(m.errors) && m.errors[m.callCount-1] != nil {
		return nil, m.errors[m.callCount-1]
	}

	if m.callCount <= len(m.responses) {
		return m.responses[m.callCount-1], nil
	}

	return nil, fmt.Errorf("unexpected call")
}

// mockTransport wraps MockHTTPClient to implement http.RoundTripper
type mockTransport struct {
	mockClient *MockHTTPClient
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.mockClient.Do(req)
}

func TestRetryableHTTPClient(t *testing.T) {
	// Test retry on 5xx errors
	t.Run("RetryOn5xx", func(t *testing.T) {
		mockClient := &MockHTTPClient{
			responses: []*http.Response{
				{StatusCode: 500},
				{StatusCode: 200},
			},
		}

		retryConfig := &RetryConfig{
			MaxRetries:      2,
			BaseDelay:       10 * time.Millisecond,
			MaxDelay:        100 * time.Millisecond,
			RetryableErrors: []int{500},
		}

		// Create a mock http.Client that uses our mock
		httpClient := &http.Client{
			Transport: &mockTransport{mockClient},
		}

		retryClient := NewRetryableHTTPClient(httpClient, retryConfig)

		req, _ := http.NewRequest("GET", "https://infrahub-api.nexgencloud.com", nil)
		resp, err := retryClient.Do(req)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resp.StatusCode != 200 {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		if mockClient.callCount != 2 {
			t.Errorf("Expected 2 calls, got %d", mockClient.callCount)
		}
	})

	// Test no retry on 4xx errors
	t.Run("NoRetryOn4xx", func(t *testing.T) {
		mockClient := &MockHTTPClient{
			responses: []*http.Response{
				{StatusCode: 400},
			},
		}

		retryConfig := DefaultRetryConfig()
		// Create a mock http.Client that uses our mock
		httpClient := &http.Client{
			Transport: &mockTransport{mockClient},
		}

		retryClient := NewRetryableHTTPClient(httpClient, retryConfig)

		req, _ := http.NewRequest("GET", "https://infrahub-api.nexgencloud.com", nil)
		resp, err := retryClient.Do(req)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}
		if mockClient.callCount != 1 {
			t.Errorf("Expected 1 call, got %d", mockClient.callCount)
		}
	})
}

func TestTimeoutConfig(t *testing.T) {
	timeoutConfig := DefaultTimeoutConfig()

	if timeoutConfig.ReadTimeout != 3*time.Second {
		t.Errorf("Expected ReadTimeout 3s, got %v", timeoutConfig.ReadTimeout)
	}

	if timeoutConfig.WriteTimeout != 15*time.Second {
		t.Errorf("Expected WriteTimeout 15s, got %v", timeoutConfig.WriteTimeout)
	}
}

func TestRetryConfig(t *testing.T) {
	retryConfig := DefaultRetryConfig()

	if retryConfig.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries 3, got %d", retryConfig.MaxRetries)
	}

	if retryConfig.BaseDelay != 100*time.Millisecond {
		t.Errorf("Expected BaseDelay 100ms, got %v", retryConfig.BaseDelay)
	}

	if retryConfig.MaxDelay != 5*time.Second {
		t.Errorf("Expected MaxDelay 5s, got %v", retryConfig.MaxDelay)
	}

	// Test retryable errors
	expectedRetryable := []int{429, 500, 502, 503, 504}
	for _, code := range expectedRetryable {
		if !retryConfig.isRetryableError(code) {
			t.Errorf("Expected %d to be retryable", code)
		}
	}

	// Test non-retryable errors
	nonRetryable := []int{400, 401, 403, 404}
	for _, code := range nonRetryable {
		if retryConfig.isRetryableError(code) {
			t.Errorf("Expected %d to not be retryable", code)
		}
	}
}

func TestWithTimeout(t *testing.T) {
	timeoutConfig := DefaultTimeoutConfig()

	// Test read timeout
	ctx, cancel := WithTimeout(context.Background(), "GET", timeoutConfig)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Error("Expected context to have deadline")
	}

	expectedDuration := timeoutConfig.ReadTimeout
	actualDuration := time.Until(deadline)

	// Allow some tolerance for test execution time
	if actualDuration < expectedDuration-time.Millisecond || actualDuration > expectedDuration+time.Millisecond {
		t.Errorf("Expected timeout around %v, got %v", expectedDuration, actualDuration)
	}

	// Test write timeout
	ctx2, cancel2 := WithTimeout(context.Background(), "POST", timeoutConfig)
	defer cancel2()

	deadline2, ok2 := ctx2.Deadline()
	if !ok2 {
		t.Error("Expected context to have deadline")
	}

	expectedDuration2 := timeoutConfig.WriteTimeout
	actualDuration2 := time.Until(deadline2)

	if actualDuration2 < expectedDuration2-time.Millisecond || actualDuration2 > expectedDuration2+time.Millisecond {
		t.Errorf("Expected timeout around %v, got %v", expectedDuration2, actualDuration2)
	}
}
