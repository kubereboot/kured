package blockers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNodeLabelFiltering(t *testing.T) {
	client := fake.NewSimpleClientset(
		&v1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-node",
				Labels: map[string]string{
					"kubernetes.io/arch":       "amd64",
					"kubernetes.io/hostname":   "test-node",
					"kubernetes.io/os":         "linux",
					"custom.label/dont-reboot": "",
				},
			},
		})

	for _, tc := range []struct {
		name        string
		filters     []string
		shouldBlock bool
	}{
		{
			name:        "doesn't block on no label filters",
			filters:     []string{},
			shouldBlock: false,
		},
		{
			name: "blocks when label exists",
			filters: []string{
				"custom.label/dont-reboot",
			},
			shouldBlock: true,
		},
		{
			name: "blocks when label value matches",
			filters: []string{
				"kubernetes.io/arch:amd64",
			},
			shouldBlock: true,
		},
		{
			name: "blocks when any matches",
			filters: []string{
				"nvidia.com/gpu.present:true",
				"kubernetes.io/arch:amd64",
			},
			shouldBlock: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			knb := KubernetesNodeBlockingChecker{
				client:   client,
				nodeName: "test-node",
			}

			for _, filter := range tc.filters {
				knb.filters = append(knb.filters, parseFilter(filter))
			}

			assert.Equal(t, tc.shouldBlock, knb.IsBlocked())
		})
	}
}

func TestNodeLabel(t *testing.T) {
	label := "kubernetes.io/os"
	val := "linux"

	for _, tc := range []struct {
		name         string
		filter       string
		shouldFilter bool
	}{
		{
			name:         "filter match on name and value",
			filter:       "kubernetes.io/os:linux",
			shouldFilter: true,
		},
		{
			name:         "filter match when label exists",
			filter:       "kubernetes.io/os",
			shouldFilter: true,
		},
		{
			name:         "filter doesn't match different label",
			filter:       "kubernetes.io/arch:amd64",
			shouldFilter: false,
		},
		{
			name:         "filter doesn't match different values",
			filter:       "kubernetes.io/os:windows",
			shouldFilter: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			filter := parseFilter(tc.filter)
			assert.Equal(t, tc.shouldFilter, filter.IsFilterMatch(label, val))
		})
	}
}
