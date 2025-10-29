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

	v1 "k8s.io/api/core/v1"
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
		return Payload{}, fmt.Errorf("failed to GET metadata: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Payload{}, fmt.Errorf("failed to read body: %w", err)
	}
	var payload Payload
	if err := json.Unmarshal(body, &payload); err != nil {
		return Payload{}, fmt.Errorf("failed to unmarshal JSON: %w", err)
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
	if response.Name == "" {
		return "", fmt.Errorf("instance hostname is empty")
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

// GetNodeObjectCountByLabel returns the number of nodes with the given label.
func GetNodeObjectCountByLabel(labelKey string) (int, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return 0, fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return 0, fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelKey,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list nodes: %v", err)
	}
	return len(nodeList.Items), nil
}

// AnnotateNodeObject annotates a node object with the given annotations.
func AnnotateNodeObject(nodeName string, annotationKey string, annotationValue string) error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %v", nodeName, err)
	}
	existingAnnotations := node.GetAnnotations()
	if existingAnnotations == nil {
		existingAnnotations = map[string]string{}
	}
	existingAnnotations[annotationKey] = annotationValue
	node.SetAnnotations(existingAnnotations)
	_, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to annotate node %s: %v", nodeName, err)
	}
	return nil
}

// CleanUpOrphanNodeObject cleans up orphan nodes by deleting the delete-candidate annotation.
func CleanUpOrphanNodeObject() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return fmt.Errorf("failed to get in-cluster config: %v", err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}
	for _, node := range nodeList.Items {
		annotations := node.GetAnnotations()
		nodeReady := true
		conditionStatus := node.Status.Conditions
		for _, condition := range conditionStatus {
			if condition.Type == v1.NodeReady {
				if condition.Status != v1.ConditionTrue {
					nodeReady = false
				}
			}
		}
		deleteCandidate := annotations[deleteCandidateAnnotation]
		if !nodeReady && deleteCandidate == "true" {
			klog.Infof("Cleaning up orphan node %s: node-ready: %t, delete-candidate: %s", node.Name, nodeReady, deleteCandidate)
			err := DeleteNodeObject([]string{node.Name})
			if err != nil {
				return fmt.Errorf("failed to delete node object%s: %v", node.Name, err)
			}
		}
	}
	return nil
}
