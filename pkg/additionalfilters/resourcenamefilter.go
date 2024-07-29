package additionalfilters

import (
	"strings"

	v1 "k8s.io/api/core/v1"
	kubectldrain "k8s.io/kubectl/pkg/drain"
)

type resourceNameFilter struct {
	filter kubectldrain.PodFilter
}

func NewResourceNameFilter(resourceNames string) *resourceNameFilter {
	// Create a custom drain filter which will be passed to the drain helper.
	// The drain helper will carry out the actual deletion of pods on a node.
	customDrainFilter := func(pod v1.Pod) kubectldrain.PodDeleteStatus {
		delete := extendedResourceFilter(pod, strings.Split(resourceNames, ","))
		if !delete {
			return kubectldrain.MakePodDeleteStatusSkip()
		}
		return kubectldrain.MakePodDeleteStatusOkay()
	}
	return &resourceNameFilter{
		filter: customDrainFilter,
	}
}

func (r *resourceNameFilter) GetPodFilter() kubectldrain.PodFilter {
	return r.filter
}

func extendedResourceFilter(pod v1.Pod, resourceNamePrefixs []string) bool {
	podHasExtendedResource := func(rl v1.ResourceList) bool {
		for resourceName := range rl {
			for _, prefix := range resourceNamePrefixs {
				if strings.HasPrefix(string(resourceName), prefix) {
					return true
				}
			}

		}
		return false
	}

	for _, c := range pod.Spec.Containers {
		if podHasExtendedResource(c.Resources.Limits) || podHasExtendedResource(c.Resources.Requests) {
			return true
		}
	}
	return false
}
