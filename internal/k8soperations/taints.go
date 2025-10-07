// Package k8soperations provides utilities to manage Kubernetes node taints for controlling
// pod scheduling and execution, drains, and other practical k8s calls.
package k8soperations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// Taint allows setting soft and hard limitations for scheduling and executing pods on nodes.
type Taint struct {
	client    *kubernetes.Clientset
	nodeID    string
	taintName string
	effect    v1.TaintEffect
	exists    bool
}

// NewTaint provides a new taint.
func NewTaint(client *kubernetes.Clientset, nodeID, taintName string, effect v1.TaintEffect) *Taint {
	exists, _, _ := taintExists(client, nodeID, taintName)

	return &Taint{
		client:    client,
		nodeID:    nodeID,
		taintName: taintName,
		effect:    effect,
		exists:    exists,
	}
}

// Enable creates the taint for a node. Creating an existing taint is a noop.
func (t *Taint) Enable() {
	if t.taintName == "" {
		return
	}

	if t.exists {
		return
	}

	preferNoSchedule(t.client, t.nodeID, t.taintName, t.effect, true)

	t.exists = true
}

// Disable removes the taint for a node. Removing a missing taint is a noop.
func (t *Taint) Disable() {
	if t.taintName == "" {
		return
	}

	if !t.exists {
		return
	}

	preferNoSchedule(t.client, t.nodeID, t.taintName, t.effect, false)

	t.exists = false
}

func taintExists(client *kubernetes.Clientset, nodeID, taintName string) (bool, int, *v1.Node) {
	updatedNode, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
	if err != nil || updatedNode == nil {
		slog.Debug("Error reading node from API server", "node", nodeID, "error", err)
		// TODO: Clarify with community if we need to exit
		os.Exit(10)
	}

	for i, taint := range updatedNode.Spec.Taints {
		if taint.Key == taintName {
			return true, i, updatedNode
		}
	}

	return false, 0, updatedNode
}

func preferNoSchedule(client *kubernetes.Clientset, nodeID, taintName string, effect v1.TaintEffect, shouldExists bool) {
	taintExists, offset, updatedNode := taintExists(client, nodeID, taintName)

	if taintExists && shouldExists {
		slog.Debug(fmt.Sprintf("Taint %v exists already for node %v.", taintName, nodeID), "node", nodeID)
		return
	}

	if !taintExists && !shouldExists {
		slog.Debug(fmt.Sprintf("Taint %v already missing for node %v.", taintName, nodeID), "node", nodeID)
		return
	}

	type patchTaints struct {
		Op    string      `json:"op"`
		Path  string      `json:"path"`
		Value interface{} `json:"value,omitempty"`
	}

	taint := v1.Taint{
		Key:    taintName,
		Effect: effect,
	}

	var patches []patchTaints

	if len(updatedNode.Spec.Taints) == 0 {
		// add first taint and ensure to keep current taints
		patches = []patchTaints{
			{
				Op:    "test",
				Path:  "/spec",
				Value: updatedNode.Spec,
			},
			{
				Op:    "add",
				Path:  "/spec/taints",
				Value: []v1.Taint{},
			},
			{
				Op:    "add",
				Path:  "/spec/taints/-",
				Value: taint,
			},
		}
	} else if taintExists {
		// remove taint and ensure to test against race conditions
		patches = []patchTaints{
			{
				Op:    "test",
				Path:  fmt.Sprintf("/spec/taints/%d", offset),
				Value: taint,
			},
			{
				Op:   "remove",
				Path: fmt.Sprintf("/spec/taints/%d", offset),
			},
		}
	} else {
		// add missing taint to existing list
		patches = []patchTaints{
			{
				Op:    "add",
				Path:  "/spec/taints/-",
				Value: taint,
			},
		}
	}

	patchBytes, err := json.Marshal(patches)
	if err != nil {
		slog.Debug(fmt.Sprintf("Error encoding taint patch for node %s: %v", nodeID, err), "node", nodeID, "error", err)
		// TODO: Clarify if we really need to exit with the community
		os.Exit(10)
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), nodeID, types.JSONPatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		// TODO: Clarify if we really need to exit with the community
		slog.Debug(fmt.Sprintf("Error patching taint for node %s: %v", nodeID, err), "node", nodeID, "error", err)
		os.Exit(10)
	}

	if shouldExists {
		slog.Info("Node taint added", "node", nodeID)
	} else {
		slog.Info("Node taint removed", "node", nodeID)
	}
}
