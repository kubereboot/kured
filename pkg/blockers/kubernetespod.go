package blockers

import (
	"context"
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"log/slog"
)

// Compile-time checks to ensure the type implements the interface
var (
	_ RebootBlocker = (*KubernetesBlockingChecker)(nil)
)

// KubernetesBlockingChecker contains info for connecting
// to k8s, and can give info about whether a reboot should be blocked
type KubernetesBlockingChecker struct {
	// client used to contact kubernetes API
	client   *kubernetes.Clientset
	nodeName string
	// lised used to filter pods (podSelector)
	filter []string
}

// NewKubernetesBlockingChecker creates a new KubernetesBlockingChecker using the provided Kubernetes client,
// node name, and pod selectors.
func NewKubernetesBlockingChecker(client *kubernetes.Clientset, nodename string, podSelectors []string) *KubernetesBlockingChecker {
	return &KubernetesBlockingChecker{
		client:   client,
		nodeName: nodename,
		filter:   podSelectors,
	}
}

// IsBlocked for the KubernetesBlockingChecker will check if a pod, for the node, is preventing
// the reboot. It will warn in the logs about blocking, but does not return an error.
func (kb KubernetesBlockingChecker) IsBlocked() bool {
	fieldSelector := fmt.Sprintf("spec.nodeName=%s,status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown", kb.nodeName)
	for _, labelSelector := range kb.filter {
		podList, err := kb.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
			Limit:         10})
		if err != nil {
			slog.Info("Reboot blocked: pod query error", "error", err)
			return true
		}

		if len(podList.Items) > 0 {
			podNames := make([]string, 0, len(podList.Items))
			for _, pod := range podList.Items {
				podNames = append(podNames, pod.Name)
			}
			if len(podList.Continue) > 0 {
				podNames = append(podNames, "...")
			}
			slog.Info(fmt.Sprintf("Reboot blocked due pods matching the patterns: %v", podNames))
			return true
		}
	}
	return false
}

func (kb KubernetesBlockingChecker) MetricLabel() string {
	return "blocker_pod"
}
