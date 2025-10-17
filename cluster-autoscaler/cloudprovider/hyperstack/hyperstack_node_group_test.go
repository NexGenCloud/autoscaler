package hyperstack

import (
	"context"
	"fmt"
	"testing"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/hyperstack/hyperstack-sdk-go"
)

type fakeClient struct{}

func (f *fakeClient) GetClusterWithResponse(_ context.Context, _ int) (*hyperstack.ClusterFields, error) {
	return &hyperstack.ClusterFields{IsReconciling: boolPtr(false), Status: strPtr("ACTIVE")}, nil
}
func (f *fakeClient) ListNodeGroupsWithResponse(_ context.Context, _ int) (*[]hyperstack.ClusterNodeGroupFields, error) {
	list := []hyperstack.ClusterNodeGroupFields{}
	return &list, nil
}
func (f *fakeClient) GetClusterNodesWithResponse(_ context.Context, _ int) (*[]hyperstack.ClusterNodeFields, error) {
	list := []hyperstack.ClusterNodeFields{}
	return &list, nil
}
func (f *fakeClient) CreateNodeWithResponse(_ context.Context, _ int, _ *int, _ *string) (*hyperstack.ClusterNodesListResponse, error) {
	return &hyperstack.ClusterNodesListResponse{}, nil
}
func (f *fakeClient) DeleteClusterNodeWithResponse(_ context.Context, _ int, _ int) (*hyperstack.ResponseModel, error) {
	return &hyperstack.ResponseModel{}, nil
}
func (f *fakeClient) DeleteClusterNodesWithResponse(_ context.Context, _ int, _ hyperstack.DeleteClusterNodesFields) (*hyperstack.ResponseModel, error) {
	return &hyperstack.ResponseModel{}, nil
}

func newTestNodeGroup(min, max, count, id int, name string) *NodeGroup {
	minPtr, maxPtr, countPtr, idPtr := intPtr(min), intPtr(max), intPtr(count), intPtr(id)
	nameCopy := name
	ngFields := &hyperstack.ClusterNodeGroupFields{
		Id:       idPtr,
		Name:     &nameCopy,
		MinCount: minPtr,
		MaxCount: maxPtr,
		Count:    countPtr,
	}
	return &NodeGroup{
		id:        id,
		minSize:   min,
		maxSize:   max,
		nodeGroup: ngFields,
		nodes:     &[]hyperstack.ClusterNodeFields{},
		manager:   &Manager{client: &fakeClient{}, nodeGroups: []*NodeGroup{}},
		clusterId: 123,
		status:    "ACTIVE",
	}
}

func intPtr(v int) *int { return &v }

func TestNodeGroup_MaxMinSize(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 2, 10, "group-a")
	if ng.MaxSize() != 5 {
		t.Fatalf("MaxSize() = %d, want 5", ng.MaxSize())
	}
	if ng.MinSize() != 1 {
		t.Fatalf("MinSize() = %d, want 1", ng.MinSize())
	}
}

func TestNodeGroup_TargetSize(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 3, 10, "group-a")
	got, err := ng.TargetSize()
	if err != nil {
		t.Fatalf("TargetSize() unexpected error: %v", err)
	}
	if got != 3 {
		t.Fatalf("TargetSize() = %d, want 3", got)
	}
}

func TestNodeGroup_IncreaseSize_Success(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 2, 10, "group-a")
	if err := ng.IncreaseSize(2); err != nil {
		t.Fatalf("IncreaseSize() unexpected error: %v", err)
	}
	if *ng.nodeGroup.Count != 4 {
		t.Fatalf("IncreaseSize() count = %d, want 4", *ng.nodeGroup.Count)
	}
}

func TestNodeGroup_IncreaseSize_TooLarge(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 4, 10, "group-a")
	if err := ng.IncreaseSize(5); err == nil {
		t.Fatalf("IncreaseSize() error = nil, want error when exceeding max size")
	}
}

func TestNodeGroup_DeleteNodes_ReconcilingSkip(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 3, 10, "group-a")
	// Force reconciling path by making manager.nodeGroups empty
	ng.manager.nodeGroups = []*NodeGroup{}
	if err := ng.DeleteNodes([]*apiv1.Node{{}}); err != nil {
		t.Fatalf("DeleteNodes() unexpected error in reconciling skip path: %v", err)
	}
}

func TestNodeGroup_DeleteNodes_MissingLabelError(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 3, 10, "group-a")
	// Ensure we do not hit reconciling early-return
	ng.manager.nodeGroups = []*NodeGroup{ng}
	node := &apiv1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n1", Labels: map[string]string{}}}
	if err := ng.DeleteNodes([]*apiv1.Node{node}); err == nil {
		t.Fatalf("DeleteNodes() error = nil, want error for missing %q label", nodeIdLabel)
	}
}

func TestNodeGroup_IdAndDebug(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 2, 42, "group-x")
	if id := ng.Id(); id != "42" {
		t.Fatalf("Id() = %q, want \"42\"", id)
	}
	dbg := ng.Debug()
	wantSub := "node group ID: 42 (min:1 max:5)"
	if dbg != wantSub {
		t.Fatalf("Debug() = %q, want %q", dbg, wantSub)
	}
}

func TestNodeGroup_Nodes(t *testing.T) {
	id1, id2 := 100, 200
	nodes := []hyperstack.ClusterNodeFields{{Id: &id1}, {Id: &id2}}
	ng := newTestNodeGroup(1, 5, 2, 42, "group-x")
	ng.nodes = &nodes
	ng.status = "ACTIVE"
	instances, err := ng.Nodes()
	if err != nil {
		t.Fatalf("Nodes() unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("Nodes() len = %d, want 2", len(instances))
	}
	if instances[0].Id != fmt.Sprintf("%d", id1) {
		t.Fatalf("Nodes()[0].Id = %q, want %d", instances[0].Id, id1)
	}
}

func TestNodeGroup_Exist(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 2, 10, "group-a")
	if !ng.Exist() {
		t.Fatalf("Exist() = false, want true")
	}
	ng.nodeGroup = nil
	if ng.Exist() {
		t.Fatalf("Exist() = true, want false")
	}
}

func TestNodeGroup_DecreaseTargetSize_NoOp(t *testing.T) {
	ng := newTestNodeGroup(1, 5, 2, 10, "group-a")
	if err := ng.DecreaseTargetSize(-1); err != nil {
		t.Fatalf("DecreaseTargetSize() unexpected error: %v", err)
	}
}

func TestFromHyperstackStatus(t *testing.T) {
	if got := fromHyperstackStatus("ACTIVE"); got != cloudprovider.InstanceRunning {
		t.Fatalf("fromHyperstackStatus(ACTIVE) unexpected value: %v", got)
	}
}

func boolPtr(b bool) *bool    { return &b }
func strPtr(s string) *string { return &s }
