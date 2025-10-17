package hyperstack

import (
	"context"
	"net/http"
	"os"
	"testing"

	hyperstack "k8s.io/autoscaler/cluster-autoscaler/cloudprovider/hyperstack/hyperstack-sdk-go"
)

func TestNewHyperstackClient_NoAPIKey(t *testing.T) {
	os.Unsetenv("HYPERSTACK_API_KEY")
	os.Unsetenv("HYPERSTACK_API_SERVER")
	if _, err := NewHyperstackClient(); err == nil {
		t.Fatalf("NewHyperstackClient() error = nil, want error when API key missing")
	}
}

func TestNewHyperstackClient_DefaultServer(t *testing.T) {
	os.Setenv("HYPERSTACK_API_KEY", "key-123")
	os.Unsetenv("HYPERSTACK_API_SERVER")
	t.Cleanup(func() {
		os.Unsetenv("HYPERSTACK_API_KEY")
	})
	c, err := NewHyperstackClient()
	if err != nil {
		t.Fatalf("NewHyperstackClient() unexpected error: %v", err)
	}
	if c.ApiServer == "" {
		t.Fatalf("ApiServer is empty, want default non-empty value")
	}
	if c.ApiKey != "key-123" {
		t.Fatalf("ApiKey = %q, want %q", c.ApiKey, "key-123")
	}
}

func TestNewHyperstackClient_WithServer(t *testing.T) {
	os.Setenv("HYPERSTACK_API_KEY", "abc")
	os.Setenv("HYPERSTACK_API_SERVER", "https://infrahub-api.nexgencloud.com/v1")
	t.Cleanup(func() {
		os.Unsetenv("HYPERSTACK_API_KEY")
		os.Unsetenv("HYPERSTACK_API_SERVER")
	})
	c, err := NewHyperstackClient()
	if err != nil {
		t.Fatalf("NewHyperstackClient() unexpected error: %v", err)
	}
	if c.ApiServer != "https://infrahub-api.nexgencloud.com/v1" {
		t.Fatalf("ApiServer = %q, want %q", c.ApiServer, "https://infrahub-api.nexgencloud.com/v1")
	}
}

func TestGetAddHeadersFn_AddsAPIKey(t *testing.T) {
	c := HyperstackClient{ApiKey: "secret-key"}
	fn := c.GetAddHeadersFn()
	req, _ := http.NewRequest("GET", "http://localhost", nil)
	if err := fn(context.Background(), req); err != nil {
		t.Fatalf("GetAddHeadersFn() unexpected error: %v", err)
	}
	if got := req.Header.Get("api_key"); got != "secret-key" {
		t.Fatalf("header api_key = %q, want %q", got, "secret-key")
	}
}

func TestHyperstack_Methods_ClientNil(t *testing.T) {
	h := &Hyperstack{Client: nil}
	if _, err := h.GetClusterWithResponse(context.Background(), 1); err == nil {
		t.Fatalf("GetClusterWithResponse() error = nil, want error for nil client")
	}
	if _, err := h.ListNodeGroupsWithResponse(context.Background(), 1); err == nil {
		t.Fatalf("ListNodeGroupsWithResponse() error = nil, want error for nil client")
	}
	if _, err := h.GetClusterNodesWithResponse(context.Background(), 1); err == nil {
		t.Fatalf("GetClusterNodesWithResponse() error = nil, want error for nil client")
	}
	if _, err := h.CreateNodeWithResponse(context.Background(), 1, nil, nil); err == nil {
		t.Fatalf("CreateNodeWithResponse() error = nil, want error for nil client")
	}
	if _, err := h.DeleteClusterNodeWithResponse(context.Background(), 1, 2); err == nil {
		t.Fatalf("DeleteClusterNodeWithResponse() error = nil, want error for nil client")
	}
	if _, err := h.DeleteClusterNodesWithResponse(context.Background(), 1, hyperstack.DeleteClusterNodesFields{Ids: nil}); err == nil {
		t.Fatalf("DeleteClusterNodesWithResponse() error = nil, want error for nil client")
	}
}

func TestNewManager_NoEnvError(t *testing.T) {
	os.Unsetenv("HYPERSTACK_API_KEY")
	os.Unsetenv("HYPERSTACK_API_SERVER")
	if _, err := newManager(); err == nil {
		t.Fatalf("newManager() error = nil, want error when env missing")
	}
}

func TestNewManager_Success(t *testing.T) {
	os.Setenv("HYPERSTACK_API_KEY", "abc-123")
	os.Setenv("HYPERSTACK_API_SERVER", "https://infrahub-api.nexgencloud.com/v1")
	t.Cleanup(func() {
		os.Unsetenv("HYPERSTACK_API_KEY")
		os.Unsetenv("HYPERSTACK_API_SERVER")
	})
	m, err := newManager()
	if err != nil {
		t.Fatalf("newManager() unexpected error: %v", err)
	}
	if m == nil || m.client == nil {
		t.Fatalf("newManager() returned nil manager or client")
	}
	if len(m.nodeGroups) != 0 {
		t.Fatalf("newManager() nodeGroups len = %d, want 0", len(m.nodeGroups))
	}
}
