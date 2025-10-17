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
// Package hyperstack contains Hyperstack cloud provider helpers for Kubernetes API interactions.
package hyperstack

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	metadataURLTemplate = "http://169.254.169.254/openstack/latest/meta_data.json"
)

// Payload is the metadata payload returned by the instance metadata service.
type Payload struct {
	UUID          string            `json:"uuid"`
	Meta          Meta              `json:"meta"`
	PublicKeys    map[string]string `json:"public_keys"`
	Keys          []Key             `json:"keys"`
	Hostname      string            `json:"hostname"`
	Name          string            `json:"name"`
	LaunchIndex   int               `json:"launch_index"`
	AZ            string            `json:"availability_zone"`
	RandomSeed    string            `json:"random_seed"`
	ProjectID     string            `json:"project_id"`
	Devices       []any             `json:"devices"`
	DedicatedCPUs []any             `json:"dedicated_cpus"`
}

// Meta represents selected metadata attributes exposed by the provider.
type Meta struct {
	Cluster     string `json:"cluster"`
	Role        string `json:"role"`
	InfrahubKey string `json:"infrahub_key"`
}

// Key represents a public key entry in the metadata response.
type Key struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"`
}

// GetMetadata retrieves instance metadata from the metadata endpoint.
func GetMetadata() (Payload, error) {
	resp, err := http.Get(metadataURLTemplate)
	if err != nil {
		panic(fmt.Errorf("failed to GET metadata: %w", err))
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(fmt.Errorf("failed to read body: %w", err))
	}
	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		panic(fmt.Errorf("failed to unmarshal JSON: %w", err))
	}
	return payload, nil
}

// GetNodeLabel returns a label value for the current node given a label key.
func GetNodeLabel(labelKey string) (string, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	response, err := GetMetadata()
	if err != nil {
		return "", fmt.Errorf("failed to get metadata: %v", err)
	}
	instanceHostname := response.Name
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), instanceHostname, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get node %s: %v", instanceHostname, err)
	}
	value, ok := node.Labels[labelKey]
	if !ok {
		return "", fmt.Errorf("label %s not found on node %s", labelKey, instanceHostname)
	}
	return value, nil
}

// DeleteNodeObject deletes Kubernetes Node objects by their names.
func DeleteNodeObject(nodeNames []string) error {
	klog.Infof("Deleting node objects: %v", nodeNames)
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	for _, nodeName := range nodeNames {
		err := clientset.CoreV1().Nodes().Delete(context.TODO(), nodeName, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete node %s: %v", nodeName, err)
		}
	}
	return nil
}
