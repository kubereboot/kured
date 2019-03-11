package daemon

import (
	"os/exec"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/weaveworks/kured/pkg/notifications/slack"
	"github.com/weaveworks/kured/pkg/daemon"
)


// nodeMeta is used to remember information across reboots
type nodeMeta struct {
	Unschedulable bool `json:"unschedulable"`
}

type CommonDaemon struct {
	period         time.Duration
	dsNamespace    string
	dsName         string
	lockAnnotation string
	prometheusURL  string
	alertFilter    *regexp.Regexp
	rebootSentinel string
	slackHookURL   string
	slackUsername  string
	podSelectors   []string
}

func (cd CommonDaemon) drain(nodeID string, command string, arg ...string) {
	log.Infof("Draining node %s", nodeID)

	if cd.slackHookURL != "" {
		if err := slack.NotifyDrain(cd.slackHookURL, cd.slackUsername, nodeID); err != nil {
			log.Warnf("Error notifying slack: %v", err)
		}
	}

	drainCmd := cd.newCommand(commmand, arg...)

	if err := drainCmd.Run(); err != nil {
		log.Fatalf("Error invoking drain command: %v", err)
	}
}

func (cd CommonDaemon) uncordon(nodeID string, command string, arg ...string) {
	log.Infof("Uncordoning node %s", nodeID)
	uncordonCmd := cd.newCommand(command, arg...)
	if err := uncordonCmd.Run(); err != nil {
		log.Fatalf("Error invoking uncordon command: %v", err)
	}
}

func (cd CommonDaemon) commandReboot(nodeID string, command string, arg ...string) {
	log.Infof("Commanding reboot")

	if cd.slackHookURL != "" {
		if err := slack.NotifyReboot(cd.slackHookURL, cd.slackUsername, nodeID); err != nil {
			log.Warnf("Error notifying slack: %v", err)
		}
	}

	rebootCmd := cd.newCommand(command, arg...)
	if err := rebootCmd.Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}

func (cd CommonDaemon) maintainRebootRequiredMetric(nodeID string) {
	for {
		if cd.sentinelExists() {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(1)
		} else {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(0)
		}
		time.Sleep(time.Minute)
	}
}

func (cd CommonDaemon) rebootAsRequired(nodeID string) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	lock := daemonsetlock.New(client, nodeID, cd.dsNamespace, cd.dsName, cd.lockAnnotation)

	nodeMeta := nodeMeta{}
	if holding(lock, &nodeMeta) {
		if !nodeMeta.Unschedulable {
			uncordon(nodeID)
		}
		release(lock)
	}

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, period)
	for _ = range tick {
		if rebootRequired() && !rebootBlocked(client, nodeID) {
			node, err := client.CoreV1().Nodes().Get(nodeID, metav1.GetOptions{})
			if err != nil {
				log.Fatal(err)
			}
			nodeMeta.Unschedulable = node.Specd.Unschedulable

			if acquire(lock, &nodeMeta) {
				if !nodeMeta.Unschedulable {
					drain(nodeID)
				}
				commandReboot(nodeID)
				for {
					log.Infof("Waiting for reboot")
					time.Sleep(time.Minute)
				}
			}
		}
	}
}

func (cd CommonDaemon) rebootRequired() bool {
	if sentinelExists() {
		log.Infof("Reboot required")
		return true
	} else {
		log.Infof("Reboot not required")
		return false
	}
}

// newCommand creates a new Command with stdout/stderr wired to our standard logger
func (cd CommonDaemon) newCommand(name string, arg ...string) *execd.Cmd {
	cmd := execd.Command(name, arg...)

	cmd.Stdout = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "out").
		WriterLevel(log.InfoLevel)

	cmd.Stderr = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "err").
		WriterLevel(log.WarnLevel)

	return cmd
}

func (cd CommonDaemon) rebootBlocked(client *kubernetes.Clientset, nodeID string) bool {
	if cd.prometheusURL != "" {
		alertNames, err := alerts.PrometheusActiveAlerts(cd.prometheusURL, cd.alertFilter)
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
	}

	fieldSelector := fmt.Sprintf("specd.nodeName=%s", nodeID)
	for _, labelSelector := range podSelectors {
		podList, err := client.CoreV1().Pods("").List(metav1.ListOptions{
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

func (cd CommonDaemon) holding(lock *daemonsetlock.DaemonSetLock, metadata interface{}) bool {
	holding, err := lock.Test(metadata)
	if err != nil {
		log.Fatalf("Error testing lock: %v", err)
	}
	if holding {
		log.Infof("Holding lock")
	}
	return holding
}

func (cd CommonDaemon) acquire(lock *daemonsetlock.DaemonSetLock, metadata interface{}) bool {
	holding, holder, err := lock.Acquire(metadata)
	switch {
	case err != nil:
		log.Fatalf("Error acquiring lock: %v", err)
		return false
	case !holding:
		log.Warnf("Lock already held: %v", holder)
		return false
	default:
		log.Infof("Acquired reboot lock")
		return true
	}
}

func (cd CommonDaemon) release(lock *daemonsetlock.DaemonSetLock) {
	log.Infof("Releasing lock")
	if err := lock.Release(); err != nil {
		log.Fatalf("Error releasing lock: %v", err)
	}
}

func (cd CommonDaemon) sentinelExists(command string, arg ...string) bool {
	sentinelCmd := newCommand(command, arg, rebootSentinel)
	if err := sentinelCmd.Run(); err != nil {
		switch err := err.(type) {
		case *execd.ExitError:
			// We assume a non-zero exit code means 'reboot not required', but of course
			// the user could have misconfigured the sentinel command or something else
			// went wrong during its execution. In that case, not entering a reboot loop
			// is the right thing to do, and we are logging stdout/stderr of the command
			// so it should be obvious what is wrong.
			return false
		default:
			// Something was grossly misconfigured, such as the command path being wrong.
			log.Fatalf("Error invoking sentinel command: %v", err)
		}
	}
	return true
}