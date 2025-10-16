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
	"strconv"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/hyperstack/hyperstack-sdk-go"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/simulator/framework"
	"k8s.io/klog/v2"
)

const (
	nodeIdLabel    = "hyperstack.cloud/node-id"
	nodeRoleLabel  = "node-role.kubernetes.io/worker"
	nodeGroupLabel = "hyperstack.cloud/node-group-id"
)

type NodeGroup struct {
	// client    hyperstackNodeGroupClient
	id        int
	minSize   int
	maxSize   int
	nodeGroup *hyperstack.ClusterNodeGroupFields
	nodes     *[]hyperstack.ClusterNodeFields
	manager   *Manager
	clusterId int
	status    string
}

// NodeGroup contains configuration info and functions to control a set
// of nodes that have the same capacity and set of labels.
// MaxSize returns maximum size of the node group.
func (n *NodeGroup) MaxSize() int {
	return n.maxSize
}

// MinSize returns minimum size of the node group.
func (n *NodeGroup) MinSize() int {
	return n.minSize
}

// TargetSize returns the current target size of the node group. It is possible that the
// number of nodes in Kubernetes is different at the moment but should be equal
// to Size() once everything stabilizes (new nodes finish startup and registration or
// removed nodes are deleted completely). Implementation required.
func (n *NodeGroup) TargetSize() (int, error) {
	klog.Info("==== TargetSize === \nn.nodeGroup.Count: ", *n.nodeGroup.Count)
	return *n.nodeGroup.Count, nil
}

// IncreaseSize increases the size of the node group. To delete a node you need
// to explicitly name it and use DeleteNode. This function should wait until
// node group size is updated. Implementation required.
func (n *NodeGroup) IncreaseSize(delta int) error {
	klog.Infof("Increasing size of node group %s by %d", *n.nodeGroup.Name, delta)
	ctx := context.Background()
	targetSize := *n.nodeGroup.Count + delta
	if targetSize > n.MaxSize() {
		return fmt.Errorf("size increase is too large. current: %d desired: %d max: %d",
			*n.nodeGroup.Count, targetSize, n.MaxSize())
	}
	klog.Infof("Creating node with target size: %d\n", targetSize)
	cloud := n.manager.client
	_, err := cloud.CreateNodeWithResponse(ctx, n.clusterId, &delta, n.nodeGroup.Name)
	if err != nil {
		return err
	}
	n.nodeGroup.Count = &targetSize
	return nil
}

// AtomicIncreaseSize tries to increase the size of the node group atomically.
// It returns error if requesting the entire delta fails. The method doesn't wait until the new instances appear.
// Implementation is optional. Implementation of this method generally requires external cloud provider support
// for atomically requesting multiple instances. If implemented, CA will take advantage of the method while scaling up
// BestEffortAtomicScaleUp ProvisioningClass, guaranteeing that all instances required for such a
// ProvisioningRequest are provisioned atomically.
func (n *NodeGroup) AtomicIncreaseSize(delta int) error {
	return cloudprovider.ErrNotImplemented
}

// DeleteNodes deletes nodes from this node group. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated. Implementation required.
func (n *NodeGroup) DeleteNodes(nodes []*apiv1.Node) error {
	if len(n.manager.nodeGroups) == 0 {
		klog.V(4).Info("[DeleteNodes] Skipping DeleteNodes, cluster is reconciling")
		return nil
	}
	ctx := context.Background()
	cloud := n.manager.client
	nodeIDsInt := make([]int, 0)
	nodeNames := make([]string, 0)
	for _, node := range nodes {
		nodeID, ok := node.Labels[nodeIdLabel]
		nodeRole := node.Labels[nodeRoleLabel]
		if !ok {
			return fmt.Errorf("node %s does not have a node ID label", node.Name)
		}
		if nodeRole != "worker" {
			klog.V(4).Infof("[DeleteNodes] Node %s is not a worker node, skipping", node.Name)
			continue
		}
		klog.V(4).Info("[DeleteNodes] Deleting node with arguments ", nodeID)
		nodeIDInt, err := strconv.Atoi(nodeID)
		if err != nil {
			return err
		}
		nodeIDsInt = append(nodeIDsInt, nodeIDInt)
		nodeNames = append(nodeNames, node.Name)
	}
	nodeIDs := hyperstack.DeleteClusterNodesFields{
		Ids: &nodeIDsInt,
	}

	_, err := cloud.DeleteClusterNodesWithResponse(ctx, n.clusterId, nodeIDs)
	if err != nil {
		return err
	}
	*n.nodeGroup.Count = *n.nodeGroup.Count - len(nodeIDsInt)
	err = DeleteNodeObject(nodeNames)
	if err != nil {
		return err
	}
	return nil
}

// ForceDeleteNodes deletes nodes from this node group, without checking for
// constraints like minimal size validation etc. Error is returned either on
// failure or if the given node doesn't belong to this node group. This function
// should wait until node group size is updated.
func (n *NodeGroup) ForceDeleteNodes([]*apiv1.Node) error {
	return cloudprovider.ErrNotImplemented
}

// DecreaseTargetSize decreases the target size of the node group. This function
// doesn't permit to delete any existing node and can be used only to reduce the
// request for new nodes that have not been yet fulfilled. Delta should be negative.
// It is assumed that cloud provider will not delete the existing nodes when there
// is an option to just decrease the target. Implementation required.
func (n *NodeGroup) DecreaseTargetSize(delta int) error {
	return nil
}

// Id returns an unique identifier of the node group.
func (n *NodeGroup) Id() string {
	klog.V(4).Info("==== Id === \nn.nodeGroup.Id: ", *n.nodeGroup.Id)
	id := strconv.Itoa(*n.nodeGroup.Id)
	return id
}

// Debug returns a string containing all information regarding this node group.
func (n *NodeGroup) Debug() string {
	return fmt.Sprintf("node group ID: %s (min:%d max:%d)", n.Id(), n.MinSize(), n.MaxSize())
}

// Nodes returns a list of all nodes that belong to this node group.
// It is required that Instance objects returned by this method have Id field set.
// Other fields are optional.
// This list should include also instances that might have not become a kubernetes node yet.
func (n *NodeGroup) Nodes() ([]cloudprovider.Instance, error) {
	klog.V(4).Info("==== Nodes === \nn.nodes: ")
	nodes := make([]cloudprovider.Instance, 0)
	for _, node := range *n.nodes {
		nodes = append(nodes, cloudprovider.Instance{
			Id: strconv.Itoa(*node.Id),
			Status: &cloudprovider.InstanceStatus{
				State: fromHyperstackStatus(n.status),
			},
		})
	}
	return nodes, nil
}

func fromHyperstackStatus(status string) cloudprovider.InstanceState {
	switch status {
	case "ACTIVE":
		return cloudprovider.InstanceRunning
	case "CREATING", "RECONCILING", "WAITING":
		return cloudprovider.InstanceCreating
	case "DELETED":
		return cloudprovider.InstanceDeleting
	default:
		return -1 // unknown status
	}
}

// TemplateNodeInfo returns a framework.NodeInfo structure of an empty
// (as if just started) node. This will be used in scale-up simulations to
// predict what would a new node look like if a node group was expanded. The returned
// NodeInfo is expected to have a fully populated Node object, with all of the labels,
// capacity and allocatable information as well as all pods that are started on
// the node by default, using manifest (most likely only kube-proxy). Implementation optional.
func (n *NodeGroup) TemplateNodeInfo() (*framework.NodeInfo, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// Exist checks if the node group really exists on the cloud provider side. Allows to tell the
// theoretical node group from the real one. Implementation required.
func (n *NodeGroup) Exist() bool {
	return n.nodeGroup != nil
}

// Create creates the node group on the cloud provider side. Implementation optional.
func (n *NodeGroup) Create() (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

// Delete deletes the node group on the cloud provider side.
// This will be executed only for autoprovisioned node groups, once their size drops to 0.
// Implementation optional.
func (n *NodeGroup) Delete() error {
	return cloudprovider.ErrNotImplemented
}

// Autoprovisioned returns true if the node group is autoprovisioned. An autoprovisioned group
// was created by CA and can be deleted when scaled to 0.
func (n *NodeGroup) Autoprovisioned() bool {
	return false
}

// GetOptions returns NodeGroupAutoscalingOptions that should be used for this particular
// NodeGroup. Returning a nil will result in using default options.
// Implementation optional. Callers MUST handle `cloudprovider.ErrNotImplemented`.
func (n *NodeGroup) GetOptions(defaults config.NodeGroupAutoscalingOptions) (*config.NodeGroupAutoscalingOptions, error) {
	return nil, cloudprovider.ErrNotImplemented
}
