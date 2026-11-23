package blockers

import (
	"context"
	"strings"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// nodeFilter is a Kubernetes label that can optionally have a value
type nodeFilter struct {
	name  string
	value string
}

// IsFilterMatch determines if a requested label filter matches our node label.
func (nf nodeFilter) IsFilterMatch(label, val string) bool {
	match := false

	if label == nf.name {
		match = true

		if nf.value != "" && val != nf.value {
			match = false
		}
	}

	return match
}

// parseFilter gets a nodeFilter from an input string
func parseFilter(label string) nodeFilter {
	parts := strings.Split(label, ":")

	nf := nodeFilter{
		name: strings.TrimSpace(parts[0]),
	}

	if len(parts) > 1 {
		nf.value = strings.TrimSpace(parts[1])
	}

	return nf
}

// Compile-time checks to ensure the type implements the interface
var (
	_ RebootBlocker = (*KubernetesNodeBlockingChecker)(nil)
)

// KubernetesNodeBlockingChecker contains info for connecting
// to k8s, and can give info about whether a reboot should be blocked
// based on the presence of node labels.
type KubernetesNodeBlockingChecker struct {
	// client used to contact kubernetes API
	client   kubernetes.Interface
	nodeName string
	// matching node labels to block
	filters []nodeFilter
}

// NewKubernetesNodeBlockingChecker creates a new KubernetesNodeBlockingChecker using the provided Kubernetes client,
// node name, and label selectors.
func NewKubernetesNodeBlockingChecker(client kubernetes.Interface, nodename string, labelSelectors []string) *KubernetesNodeBlockingChecker {
	knb := &KubernetesNodeBlockingChecker{
		client:   client,
		nodeName: nodename,
	}

	for _, filter := range labelSelectors {
		nodeFilter := parseFilter(filter)
		knb.filters = append(knb.filters, nodeFilter)
	}

	return knb
}

// IsBlocked for the KubernetesNodeBlockingChecker will check if a node has matching labels and
// if so, block the reboot. It will warn in the logs about blocking, but does not return an error.
func (knb KubernetesNodeBlockingChecker) IsBlocked() bool {
	// Avoid making calls if nothing has been set to filter on
	if len(knb.filters) == 0 {
		return false
	}

	// Get this node to inspect its labels
	node, err := knb.client.CoreV1().Nodes().Get(context.Background(), knb.nodeName, metav1.GetOptions{})
	if err != nil {
		log.Warnf("Reboot blocked: node query error: %v", err)
		return true
	}

	// Check against each provided filter to see if it matches what is set
	for label, val := range node.Labels {
		for _, filter := range knb.filters {
			if filter.IsFilterMatch(label, val) {
				log.Warnf("Reboot blocked: matching labels: %s:%s", label, val)
				return true
			}
		}
	}

	return false
}
