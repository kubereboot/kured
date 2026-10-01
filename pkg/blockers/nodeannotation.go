package blockers

import (
	"context"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Compile-time checks to ensure the type implements the interface
var (
	_ RebootBlocker = (*NodeBlockingChecker)(nil)
)

// NodeBlockingChecker contains info for connecting
// to k8s, and can give info about whether a reboot should be blocked
type NodeBlockingChecker struct {
	// client used to contact kubernetes API
	client   *kubernetes.Clientset
	nodeName string
	// lised used to filter pods (podSelector)
	filter []string
}

func NewNodeBlockingChecker(client *kubernetes.Clientset, nodename string, nodeAnnotations []string) *NodeBlockingChecker {
	return &NodeBlockingChecker{
		client:   client,
		nodeName: nodename,
		filter:   nodeAnnotations,
	}
}

// IsBlocked for the NodeBlockingChecker will check if a pod, for the node, is preventing
// the reboot. It will warn in the logs about blocking, but does not return an error.
func (kb *NodeBlockingChecker) IsBlocked() bool {
	node, err := kb.client.CoreV1().Nodes().Get(context.TODO(), kb.nodeName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Reboot blocked: node query error: %v", err)
		return true
	}
	for _, annotation := range kb.filter {
		if _, exists := node.Annotations[annotation]; exists {
			log.Warnf("Reboot blocked: node annotation %s exists.", annotation)
			return true
		}
	}
	return false
}
