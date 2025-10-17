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
	"os"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
)

func TestName(t *testing.T) {
	p := newHyperstackCloudProvider(&Manager{}, &cloudprovider.ResourceLimiter{})
	if got, want := p.Name(), cloudprovider.HyperstackProviderName; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}
}

func TestNodeGroups(t *testing.T) {
	mg := &Manager{nodeGroups: []*NodeGroup{{id: 1}, {id: 2}}}
	p := newHyperstackCloudProvider(mg, &cloudprovider.ResourceLimiter{})

	groups := p.NodeGroups()
	if len(groups) != 2 {
		t.Fatalf("NodeGroups() length = %d, want 2", len(groups))
	}
}

func TestNodeGroupForNode(t *testing.T) {
	mg := &Manager{nodeGroups: []*NodeGroup{{id: 10}, {id: 20}}}
	p := newHyperstackCloudProvider(mg, &cloudprovider.ResourceLimiter{})

	// Found case
	node := &apiv1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{nodeGroupLabel: "20"}}}
	ng, err := p.NodeGroupForNode(node)
	if err != nil {
		t.Fatalf("NodeGroupForNode() unexpected error: %v", err)
	}
	if ng == nil {
		t.Fatalf("NodeGroupForNode() = nil, want non-nil")
	}

	// Missing label -> nil, nil
	node2 := &apiv1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}}}
	ng, err = p.NodeGroupForNode(node2)
	if err != nil {
		t.Fatalf("NodeGroupForNode() unexpected error (no label): %v", err)
	}
	if ng != nil {
		t.Fatalf("NodeGroupForNode() = %v, want nil when label missing", ng)
	}

	// Non-integer label -> error
	node3 := &apiv1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{nodeGroupLabel: "abc"}}}
	_, err = p.NodeGroupForNode(node3)
	if err == nil {
		t.Fatalf("NodeGroupForNode() error = nil, want non-nil for non-integer label")
	}
}
func TestBuildHyperstack(t *testing.T) {
	// Failure: missing API key
	os.Unsetenv("HYPERSTACK_API_KEY")
	if got := BuildHyperstack(config.AutoscalingOptions{}, cloudprovider.NodeGroupDiscoveryOptions{}, &cloudprovider.ResourceLimiter{}); got != nil {
		t.Fatalf("BuildHyperstack() without API key = %v, want nil", got)
	}

	// Success: with API key
	os.Setenv("HYPERSTACK_API_KEY", "abc-123-xyz")
	os.Setenv("HYPERSTACK_API_SERVER", "https://infrahub-api.nexgencloud.com/v1")
	t.Cleanup(func() {
		os.Unsetenv("HYPERSTACK_API_KEY")
		os.Unsetenv("HYPERSTACK_API_SERVER")
	})
	got := BuildHyperstack(config.AutoscalingOptions{}, cloudprovider.NodeGroupDiscoveryOptions{}, &cloudprovider.ResourceLimiter{})
	if got == nil {
		t.Fatalf("BuildHyperstack() with API key = nil, want non-nil provider")
	}
}
