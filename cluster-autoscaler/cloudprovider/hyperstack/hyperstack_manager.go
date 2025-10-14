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
	"os"
	"strconv"

	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/hyperstack/hyperstack-sdk-go"
	"k8s.io/klog/v2"
)

const (
	clusterIdLabel = "hyperstack.cloud/cluster-id"
)

type HyperstackClient struct {
	Client    *http.Client
	ApiKey    string
	ApiServer string
}

type hyperstackNodeGroupClient interface {
	GetClusterWithResponse(ctx context.Context, clusterId int) (*hyperstack.ClusterFields, error)
	ListNodeGroupsWithResponse(ctx context.Context, clusterId int) (*[]hyperstack.ClusterNodeGroupFields, error)
	GetClusterNodesWithResponse(ctx context.Context, clusterId int) (*[]hyperstack.ClusterNodeFields, error)
	CreateNodeWithResponse(ctx context.Context, clusterId int, count *int, nodeGroup *string) (*hyperstack.ClusterNodesListResponse, error)
	DeleteClusterNodeWithResponse(ctx context.Context, clusterId int, nodeId int) (*hyperstack.ResponseModel, error)
	DeleteClusterNodesWithResponse(ctx context.Context, clusterId int, nodeIds hyperstack.DeleteClusterNodesFields) (*hyperstack.ResponseModel, error)
}

type Hyperstack struct {
	Client *HyperstackClient
}
type Manager struct {
	client     hyperstackNodeGroupClient
	nodeGroups []*NodeGroup
}

func newManager() (*Manager, error) {
	client, err := NewHyperstackClient()
	if err != nil {
		return nil, err
	}
	return &Manager{
		client:     &Hyperstack{Client: client},
		nodeGroups: make([]*NodeGroup, 0),
	}, nil
}

func NewHyperstackClient() (*HyperstackClient, error) {
	apiKey := os.Getenv("HYPERSTACK_API_KEY")
	apiServer := os.Getenv("HYPERSTACK_API_SERVER")
	if apiKey == "" {
		return nil, fmt.Errorf("api key is not provided")
	}
	if apiServer == "" {
		apiServer = "https://infrahub-api.nexgencloud.com/v1"
	}
	return &HyperstackClient{
		Client:    http.DefaultClient,
		ApiKey:    apiKey,
		ApiServer: apiServer,
	}, nil
}

func (c HyperstackClient) GetAddHeadersFn() func(ctx context.Context, req *http.Request) error {
	return func(ctx context.Context, req *http.Request) error {
		req.Header.Add("api_key", c.ApiKey)
		return nil
	}
}

func (h *Hyperstack) GetClusterWithResponse(ctx context.Context, clusterId int) (*hyperstack.ClusterFields, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("hyperstack client is not initialized")
	}
	client, err := hyperstack.NewClientWithResponses(h.Client.ApiServer, hyperstack.WithRequestEditorFn(h.Client.GetAddHeadersFn()))
	if err != nil {
		return nil, err
	}
	result, err := client.GettingClusterDetailWithResponse(ctx, clusterId)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("empty response from GettingClusterDetailWithResponse")
	}
	if result.JSON400 != nil {
		return nil, fmt.Errorf("error code: %d", result.StatusCode())
	}
	if result.JSON401 != nil {
		return nil, fmt.Errorf("error code: %d", result.StatusCode())
	}
	if result.JSON404 != nil {
		return nil, fmt.Errorf("error code: %d", result.StatusCode())
	}
	if result.JSON200 == nil {
		return nil, fmt.Errorf("result is nil (status code: %d)", result.StatusCode())
	}
	return result.JSON200.Cluster, nil
}

func (h *Hyperstack) ListNodeGroupsWithResponse(ctx context.Context, clusterId int) (*[]hyperstack.ClusterNodeGroupFields, error) {
	if h.Client == nil {
		return nil, fmt.Errorf("hyperstack client is not initialized")
	}
	client, err := hyperstack.NewClientWithResponses(h.Client.ApiServer, hyperstack.WithRequestEditorFn(h.Client.GetAddHeadersFn()))
	if err != nil {
		return nil, err
	}
	klog.V(4).Infof("Making list node groups API call to %s for cluster ID %d", h.Client.ApiServer, clusterId)
	result, err := client.ListNodeGroupsWithResponse(ctx, clusterId)
	if err != nil {
		klog.Errorf("API call failed with error: %v", err)
		return nil, err
	}
	klog.V(4).Infof("API Response Status: ")
	if result == nil {
		return nil, fmt.Errorf("empty response from ListNodeGroupsWithResponse")
	}
	if result.JSON400 != nil {
		errorReason := "unknown error"
		if result.JSON400.ErrorReason != nil {
			errorReason = *result.JSON400.ErrorReason
		}
		return nil, fmt.Errorf("error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON401 != nil {
		errorReason := "unknown error"
		if result.JSON401.ErrorReason != nil {
			errorReason = *result.JSON401.ErrorReason
		}
		return nil, fmt.Errorf("error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON404 != nil {
		errorReason := "unknown error"
		if result.JSON404.ErrorReason != nil {
			errorReason = *result.JSON404.ErrorReason
		}
		return nil, fmt.Errorf("error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON200 == nil {
		return nil, fmt.Errorf("result is nil (status code: %d)", result.StatusCode())
	}
	list := result.JSON200.NodeGroups
	return list, nil
}
func (h *Hyperstack) CreateNodeWithResponse(ctx context.Context, clusterId int, count *int, nodeGroup *string) (*hyperstack.ClusterNodesListResponse, error) {
	klog.V(4).Info("[CreateNodeWithResponse] Creating node with arguments ", clusterId, count, nodeGroup)
	if h.Client == nil {
		return nil, fmt.Errorf("[CreateNodeWithResponse] Hyperstack client is not initialized")
	}
	client, err := hyperstack.NewClientWithResponses(h.Client.ApiServer, hyperstack.WithRequestEditorFn(h.Client.GetAddHeadersFn()))
	if err != nil {
		return nil, err
	}
	role := hyperstack.CreateClusterNodeFieldsRoleWorker
	body := hyperstack.CreateClusterNodeFields{
		Count:     count,
		NodeGroup: nodeGroup,
		Role:      &role,
	}
	result, err := client.CreateNodeWithResponse(ctx, clusterId, body)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, fmt.Errorf("[CreateNodeWithResponse] Empty response from CreateNodeWithResponse")
	}
	if result.JSON400 != nil {
		errorReason := "unknown error"
		if result.JSON400.ErrorReason != nil {
			errorReason = *result.JSON400.ErrorReason
		}
		return nil, fmt.Errorf("[CreateNodeWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON401 != nil {
		errorReason := "unknown error"
		if result.JSON401.ErrorReason != nil {
			errorReason = *result.JSON401.ErrorReason
		}
		return nil, fmt.Errorf("[CreateNodeWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON404 != nil {
		errorReason := "unknown error"
		if result.JSON404.ErrorReason != nil {
			errorReason = *result.JSON404.ErrorReason
		}
		return nil, fmt.Errorf("[CreateNodeWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON409 != nil {
		errorReason := "unknown error"
		if result.JSON409.ErrorReason != nil {
			errorReason = *result.JSON409.ErrorReason
		}
		return nil, fmt.Errorf("[CreateNodeWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON201 == nil {
		return nil, fmt.Errorf("[CreateNodeWithResponse] Result is nil (status code: %d)", result.StatusCode())
	}
	// fmt.Println(result.StatusCode(), "=====")
	return result.JSON201, nil
}
func (h *Hyperstack) DeleteClusterNodeWithResponse(ctx context.Context, clusterId int, nodeId int) (*hyperstack.ResponseModel, error) {
	klog.V(4).Info("[DeleteClusterNodeWithResponse] Deleting cluster node with arguments ", clusterId, nodeId)
	if h.Client == nil {
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Hyperstack client is not initialized")
	}
	client, err := hyperstack.NewClientWithResponses(h.Client.ApiServer, hyperstack.WithRequestEditorFn(h.Client.GetAddHeadersFn()))
	if err != nil {
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Error initializing client: %v", err)
	}
	result, err := client.DeleteClusterNodeWithResponse(ctx, clusterId, nodeId)
	if err != nil {
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Error calling DeleteClusterNode: %v", err)
	}
	if result == nil {
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Empty response from DeleteClusterNodeWithResponse")
	}
	if result.JSON400 != nil {
		errorReason := "unknown error"
		if result.JSON400.ErrorReason != nil {
			errorReason = *result.JSON400.ErrorReason
		}
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON401 != nil {
		errorReason := "unknown error"
		if result.JSON401.ErrorReason != nil {
			errorReason = *result.JSON401.ErrorReason
		}
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON404 != nil {
		errorReason := "unknown error"
		if result.JSON404.ErrorReason != nil {
			errorReason = *result.JSON404.ErrorReason
		}
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON200 == nil {
		return nil, fmt.Errorf("[DeleteClusterNodeWithResponse] Result is nil (status code: %d)", result.StatusCode())
	}
	return result.JSON200, nil
}

func (h *Hyperstack) DeleteClusterNodesWithResponse(ctx context.Context, clusterId int, nodeIds hyperstack.DeleteClusterNodesFields) (*hyperstack.ResponseModel, error) {
	klog.V(4).Info("[DeleteClusterNodesWithResponse] Deleting cluster nodes with arguments ", clusterId, nodeIds)
	if h.Client == nil {
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Hyperstack client is not initialized")
	}
	client, err := hyperstack.NewClientWithResponses(h.Client.ApiServer, hyperstack.WithRequestEditorFn(h.Client.GetAddHeadersFn()))
	if err != nil {
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Error initializing client: %v", err)
	}
	result, err := client.DeleteClusterNodesWithResponse(ctx, clusterId, nodeIds)
	if err != nil {
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Error calling DeleteClusterNode: %v", err)
	}
	if result == nil {
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Empty response from DeleteClusterNodesWithResponse")
	}
	if result.JSON400 != nil {
		errorReason := "unknown error"
		if result.JSON400.ErrorReason != nil {
			errorReason = *result.JSON400.ErrorReason
		}
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON401 != nil {
		errorReason := "unknown error"
		if result.JSON401.ErrorReason != nil {
			errorReason = *result.JSON401.ErrorReason
		}
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON404 != nil {
		errorReason := "unknown error"
		if result.JSON404.ErrorReason != nil {
			errorReason = *result.JSON404.ErrorReason
		}
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON200 == nil {
		return nil, fmt.Errorf("[DeleteClusterNodesWithResponse] Result is nil (status code: %d)", result.StatusCode())
	}
	return result.JSON200, nil

}

func (h *Hyperstack) GetClusterNodesWithResponse(ctx context.Context, clusterId int) (*[]hyperstack.ClusterNodeFields, error) {
	klog.V(4).Info("[GetClusterNodesWithResponse] Getting cluster nodes with arguments ", clusterId)
	if h.Client == nil {
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Hyperstack client is not initialized")
	}
	client, err := hyperstack.NewClientWithResponses(h.Client.ApiServer, hyperstack.WithRequestEditorFn(h.Client.GetAddHeadersFn()))
	if err != nil {
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Error initializing client: %v", err)
	}
	klog.V(4).Infof("Making GetClusterNodes API call to %s for cluster ID %d", h.Client.ApiServer, clusterId)
	result, err := client.GetClusterNodesWithResponse(ctx, clusterId)
	if err != nil {
		klog.Errorf("GetClusterNodes API call failed with error: %v", err)
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Error calling GetClusterNodes: %v", err)
	}
	klog.V(4).Infof("GetClusterNodes API Response Status")
	if result == nil {
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Empty response from GetClusterNodesWithResponse")
	}
	if result.JSON400 != nil {
		errorReason := "unknown error"
		if result.JSON400.ErrorReason != nil {
			errorReason = *result.JSON400.ErrorReason
		}
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON401 != nil {
		errorReason := "unknown error"
		if result.JSON401.ErrorReason != nil {
			errorReason = *result.JSON401.ErrorReason
		}
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON404 != nil {
		errorReason := "unknown error"
		if result.JSON404.ErrorReason != nil {
			errorReason = *result.JSON404.ErrorReason
		}
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Error reason: %s | error code: %d)", errorReason, result.StatusCode())
	}
	if result.JSON200 == nil {
		return nil, fmt.Errorf("[GetClusterNodesWithResponse] Result is nil (status code: %d)", result.StatusCode())
	}
	return result.JSON200.Nodes, nil
}

func (m *Manager) Refresh() error {
	ctx := context.Background()
	clusterId, err := GetNodeLabel(clusterIdLabel)
	if err != nil {
		return err
	}
	clusterIdInt, err := strconv.Atoi(clusterId)
	if err != nil {
		return err
	}
	nodeGroups, err := m.client.ListNodeGroupsWithResponse(ctx, clusterIdInt)
	if err != nil {
		return err
	}
	cluster, err := m.client.GetClusterWithResponse(ctx, clusterIdInt)
	if err != nil {
		return err
	}
	group := make([]*NodeGroup, 0)
	if *cluster.IsReconciling {
		return fmt.Errorf("[Refresh] Cluster is reconciling, skipping refresh")
	}
	for _, nodeGroup := range *nodeGroups {
		if *nodeGroup.Role != "worker" {
			continue
		}
		if *nodeGroup.MaxCount <= *nodeGroup.MinCount {
			klog.V(4).Infof("[Refresh] Skipping node group %d as maxCount (%d) <= minCount (%d)", *nodeGroup.Id, *nodeGroup.MaxCount, *nodeGroup.MinCount)
			continue
		}
		nodes, err := m.client.GetClusterNodesWithResponse(ctx, clusterIdInt)
		if err != nil {
			return err
		}

		klog.V(4).Infof("[Refresh] adding node group | node group id: %d | node group count: %d", *nodeGroup.Id, *nodeGroup.Count)
		group = append(group, &NodeGroup{
			id:        *nodeGroup.Id,
			minSize:   *nodeGroup.MinCount,
			maxSize:   *nodeGroup.MaxCount,
			nodeGroup: &nodeGroup,
			nodes:     nodes,
			clusterId: clusterIdInt,
			status:    *cluster.Status,
			manager:   m,
		})
	}
	m.nodeGroups = group
	return err
}
