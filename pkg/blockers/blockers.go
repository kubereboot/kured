package blockers

import (
	"context"
	"fmt"
	"github.com/kubereboot/kured/pkg/alerts"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"regexp"
)

// RebootBlocked checks that a single block Checker
// will block the reboot or not.
func RebootBlocked(blockers ...RebootBlocker) bool {
	for _, blocker := range blockers {
		if blocker.IsBlocked() {
			return true
		}
	}
	return false
}

// RebootBlocker interface should be implemented by types
// to know if their instantiations should block a reboot
type RebootBlocker interface {
	IsBlocked() bool
}

// PrometheusBlockingChecker contains info for connecting
// to prometheus, and can give info about whether a reboot should be blocked
type PrometheusBlockingChecker struct {
	// prometheusClient to make prometheus-go-client and api config available
	// into the PrometheusBlockingChecker struct
	PromClient *alerts.PromClient
	// regexp used to get alerts
	Filter *regexp.Regexp
	// bool to indicate if only firing alerts should be considered
	FiringOnly bool
	// bool to indicate that we're only blocking on alerts which match the filter
	FilterMatchOnly bool
}

// KubernetesBlockingChecker contains info for connecting
// to k8s, and can give info about whether a reboot should be blocked
type KubernetesBlockingChecker struct {
	// client used to contact kubernetes API
	Client   *kubernetes.Clientset
	Nodename string
	// lised used to filter pods (podSelector)
	Filter []string
}

// IsBlocked for the prometheus will check if there are active alerts matching
// the arguments given into promclient which would actively block the reboot.
// As of today, no blocker information is shared as a return of the method,
// and the information is simply logged.
func (pb PrometheusBlockingChecker) IsBlocked() bool {
	alertNames, err := pb.PromClient.ActiveAlerts(pb.Filter, pb.FiringOnly, pb.FilterMatchOnly)
	if err != nil {
		log.Warnf("Reboot blocked: prometheus query error: %v", err)
		return true
	}
	count := len(alertNames)
	if count > 10 {
		alertNames = append(alertNames[:10], "...")
	}
	if count > 0 {
		log.Warnf("Reboot blocked: %d active alerts: %v", count, alertNames)
		return true
	}
	return false
}

// IsBlocked for the KubernetesBlockingChecker will check if a pod, for the node, is preventing
// the reboot. It will warn in the logs about blocking, but does not return an error.
func (kb KubernetesBlockingChecker) IsBlocked() bool {
	fieldSelector := fmt.Sprintf("spec.nodeName=%s,status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown", kb.Nodename)
	for _, labelSelector := range kb.Filter {
		podList, err := kb.Client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
			FieldSelector: fieldSelector,
			Limit:         10})
		if err != nil {
			log.Warnf("Reboot blocked: pod query error: %v", err)
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
			log.Warnf("Reboot blocked: matching pods: %v", podNames)
			return true
		}
	}
	return false
}
