package k8soperations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

const (
	// KuredNodeWasUnschedulableBeforeDrainAnnotation contains is the key where kured stores whether the node was unschedulable before the maintenance.
	KuredNodeWasUnschedulableBeforeDrainAnnotation string = "kured.dev/node-unschedulable-before-drain"
)

// AddNodeAnnotations adds or updates annotations on a Kubernetes node.
// Parameters:
//   - client: Kubernetes client set for API operations
//   - nodeID: identifier of the target node
//   - annotations: map of key-value pairs to be added as annotations
//
// Returns an error if the operation fails, nil otherwise
// The intent was a generic annotation system that can be used in multiple places.
func AddNodeAnnotations(client *kubernetes.Clientset, nodeID string, annotations map[string]string) error {
	node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving node object via k8s API: %v", err)
	}
	for k, v := range annotations {
		node.Annotations[k] = v
		slog.Debug(fmt.Sprintf("adding node annotation: %s=%s", k, v), "node", node.GetName())
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("error marshalling node object into JSON: %v", err)
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), node.GetName(), types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		var annotationsErr string
		for k, v := range annotations {
			annotationsErr += fmt.Sprintf("%s=%s ", k, v)
		}
		return fmt.Errorf("error adding node annotations %s via k8s API: %v", annotationsErr, err)
	}
	return nil
}

// DeleteNodeAnnotation deletes an annotation from a Kubernetes node.
// Parameters:
//   - client: Kubernetes client set for API operations
//   - nodeID: identifier of the target node
//   - key: key of the annotation to be deleted
//
// Returns an error if the operation fails, nil otherwise
func DeleteNodeAnnotation(client *kubernetes.Clientset, nodeID, key string) error {
	// JSON Patch takes as path input a JSON Pointer, defined in RFC6901
	// So we replace all instances of "/" with "~1" as per:
	// https://tools.ietf.org/html/rfc6901#section-3
	patch := []byte(fmt.Sprintf("[{\"op\":\"remove\",\"path\":\"/metadata/annotations/%s\"}]", strings.ReplaceAll(key, "/", "~1")))
	_, err := client.CoreV1().Nodes().Patch(context.TODO(), nodeID, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error deleting node annotation %s via k8s API: %v", key, err)
	}
	return nil
}

func updateNodeLabels(client *kubernetes.Clientset, node *v1.Node, labels []string) error {
	labelsMap := make(map[string]string)
	for _, label := range labels {
		k := strings.Split(label, "=")[0]
		v := strings.Split(label, "=")[1]
		labelsMap[k] = v
		slog.Debug(fmt.Sprintf("Updating node %s label: %s=%s", node.GetName(), k, v), "node", node.GetName())
	}

	bytes, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labelsMap,
		},
	})
	if err != nil {
		return fmt.Errorf("error marshalling node object into JSON: %v", err)
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), node.GetName(), types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		var labelsErr string
		for _, label := range labels {
			k := strings.Split(label, "=")[0]
			v := strings.Split(label, "=")[1]
			labelsErr += fmt.Sprintf("%s=%s ", k, v)
		}
		return fmt.Errorf("error updating node labels %s via k8s API: %v", labelsErr, err)
	}
	return nil
}
