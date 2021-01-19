package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubectldrain "k8s.io/kubectl/pkg/drain"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/weaveworks/kured/pkg/alerts"
	"github.com/weaveworks/kured/pkg/daemonsetlock"
	"github.com/weaveworks/kured/pkg/delaytick"
	"github.com/weaveworks/kured/pkg/notifications/slack"
	"github.com/weaveworks/kured/pkg/taints"
	"github.com/weaveworks/kured/pkg/timewindow"
)

var (
	version = "unreleased"

	// Command line flags
	forceDrain                bool
	forceReboot               bool
	forceTimeout              time.Duration
	period                    time.Duration
	drainGracePeriod          int
	dsNamespace               string
	dsName                    string
	lockAnnotation            string
	lockTTL                   time.Duration
	prometheusURL             string
	alertFilter               *regexp.Regexp
	rebootSentinel            string
	preferNoScheduleTaintName string
	slackHookURL              string
	slackUsername             string
	slackChannel              string
	messageTemplateDrain      string
	messageTemplateReboot     string
	podSelectors              []string

	rebootDays  []string
	rebootStart string
	rebootEnd   string
	timezone    string

	// Metrics
	rebootRequiredGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "kured",
		Name:      "reboot_required",
		Help:      "OS requires reboot due to software updates.",
	}, []string{"node"})
)

func init() {
	prometheus.MustRegister(rebootRequiredGauge)
}

func main() {
	rootCmd := &cobra.Command{
		Use:   "kured",
		Short: "Kubernetes Reboot Daemon",
		Run:   root}

	rootCmd.PersistentFlags().BoolVar(&forceReboot, "force-reboot", false,
		"enable/disable force reboot")
	rootCmd.PersistentFlags().IntVar(&drainGracePeriod, "drain-grace-period", -1,
		"drain grace period in seconds")
	rootCmd.PersistentFlags().DurationVar(&forceTimeout, "force-timeout", time.Minute*30,
		"total timeout which only applies when force-reboot is set to true")
	rootCmd.PersistentFlags().DurationVar(&period, "period", time.Minute*60,
		"reboot check period")
	rootCmd.PersistentFlags().StringVar(&dsNamespace, "ds-namespace", "kube-system",
		"namespace containing daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&dsName, "ds-name", "kured",
		"name of daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&lockAnnotation, "lock-annotation", "weave.works/kured-node-lock",
		"annotation in which to record locking node")
	rootCmd.PersistentFlags().DurationVar(&lockTTL, "lock-ttl", 0,
		"expire lock annotation after this duration (default: 0, disabled)")
	rootCmd.PersistentFlags().StringVar(&prometheusURL, "prometheus-url", "",
		"Prometheus instance to probe for active alerts")
	rootCmd.PersistentFlags().Var(&regexpValue{&alertFilter}, "alert-filter-regexp",
		"alert names to ignore when checking for active alerts")
	rootCmd.PersistentFlags().StringVar(&rebootSentinel, "reboot-sentinel", "/var/run/reboot-required",
		"path to file whose existence signals need to reboot")
	rootCmd.PersistentFlags().StringVar(&preferNoScheduleTaintName, "prefer-no-schedule-taint", "",
		"Taint name applied during pending node reboot (to prevent receiving additional pods from other rebooting nodes). Disabled by default. Set e.g. to \"weave.works/kured-node-reboot\" to enable tainting.")

	rootCmd.PersistentFlags().StringVar(&slackHookURL, "slack-hook-url", "",
		"slack hook URL for reboot notfications")
	rootCmd.PersistentFlags().StringVar(&slackUsername, "slack-username", "kured",
		"slack username for reboot notfications")
	rootCmd.PersistentFlags().StringVar(&slackChannel, "slack-channel", "",
		"slack channel for reboot notfications")
	rootCmd.PersistentFlags().StringVar(&messageTemplateDrain, "message-template-drain", "Draining node %s",
		"message template used to notify about a node being drained")
	rootCmd.PersistentFlags().StringVar(&messageTemplateReboot, "message-template-reboot", "Rebooting node %s",
		"message template used to notify about a node being rebooted")

	rootCmd.PersistentFlags().StringArrayVar(&podSelectors, "blocking-pod-selector", nil,
		"label selector identifying pods whose presence should prevent reboots")

	rootCmd.PersistentFlags().StringSliceVar(&rebootDays, "reboot-days", timewindow.EveryDay,
		"schedule reboot on these days")
	rootCmd.PersistentFlags().StringVar(&rebootStart, "start-time", "0:00",
		"schedule reboot only after this time of day")
	rootCmd.PersistentFlags().StringVar(&rebootEnd, "end-time", "23:59:59",
		"schedule reboot only before this time of day")
	rootCmd.PersistentFlags().StringVar(&timezone, "time-zone", "UTC",
		"use this timezone for schedule inputs")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// newCommand creates a new Command with stdout/stderr wired to our standard logger
func newCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)

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

func sentinelExists() bool {
	// Relies on hostPID:true and privileged:true to enter host mount space
	sentinelCmd := newCommand("/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "/usr/bin/test", "-f", rebootSentinel)
	if err := sentinelCmd.Run(); err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
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

func rebootRequired() bool {
	if sentinelExists() {
		log.Infof("Reboot required")
		return true
	}
	log.Infof("Reboot not required")
	return false
}

func rebootBlocked(client *kubernetes.Clientset, nodeID string) bool {
	if prometheusURL != "" {
		alertNames, err := alerts.PrometheusActiveAlerts(prometheusURL, alertFilter)
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

	fieldSelector := fmt.Sprintf("spec.nodeName=%s", nodeID)
	for _, labelSelector := range podSelectors {
		podList, err := client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
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

func holding(lock *daemonsetlock.DaemonSetLock, metadata interface{}) bool {
	holding, err := lock.Test(metadata)
	if err != nil {
		log.Fatalf("Error testing lock: %v", err)
	}
	if holding {
		log.Infof("Holding lock")
	}
	return holding
}

func acquire(lock *daemonsetlock.DaemonSetLock, metadata interface{}, TTL time.Duration) bool {
	holding, holder, err := lock.Acquire(metadata, TTL)
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

func release(lock *daemonsetlock.DaemonSetLock) {
	log.Infof("Releasing lock")
	if err := lock.Release(); err != nil {
		log.Fatalf("Error releasing lock: %v", err)
	}
}

func drain(client *kubernetes.Clientset, node *v1.Node) {
	nodename := node.GetName()

	log.Infof("Draining node %s", nodename)

	if slackHookURL != "" {
		if err := slack.NotifyDrain(slackHookURL, slackUsername, slackChannel, messageTemplateDrain, nodename); err != nil {
			log.Warnf("Error notifying slack: %v", err)
		}
	}

	drainer := &kubectldrain.Helper{
		Client:              client,
		Ctx:                 context.Background(),
		GracePeriodSeconds:  drainGracePeriod,
		Force:               true,
		DeleteLocalData:     true,
		IgnoreAllDaemonSets: true,
		ErrOut:              os.Stderr,
		Out:                 os.Stdout,
	}
	if forceReboot {
		drainer.Timeout = forceTimeout
	}

	if err := kubectldrain.RunCordonOrUncordon(drainer, node, true); err != nil {
		log.Fatalf("Error cordonning %s: %v", nodename, err)
	}

	if err := kubectldrain.RunNodeDrain(drainer, nodename); err != nil {
		if !forceReboot {
			log.Fatalf("Error draining %s: %v", nodename, err)
		}
		log.Errorf("Error draining %s: %v, continuing with reboot anyway", nodename, err)
	}
}

func uncordon(client *kubernetes.Clientset, node *v1.Node) {
	nodename := node.GetName()
	log.Infof("Uncordoning node %s", nodename)
	drainer := &kubectldrain.Helper{
		Client: client,
		ErrOut: os.Stderr,
		Out:    os.Stdout,
	}
	if err := kubectldrain.RunCordonOrUncordon(drainer, node, false); err != nil {
		log.Fatalf("Error uncordonning %s: %v", nodename, err)
	}
}

func commandReboot(nodeID string) {
	log.Infof("Commanding reboot for node: %s", nodeID)

	if slackHookURL != "" {
		if err := slack.NotifyReboot(slackHookURL, slackUsername, slackChannel, messageTemplateReboot, nodeID); err != nil {
			log.Warnf("Error notifying slack: %v", err)
		}
	}

	// Relies on hostPID:true and privileged:true to enter host mount space
	rebootCmd := newCommand("/usr/bin/nsenter", "-m/proc/1/ns/mnt", "/bin/systemctl", "reboot")
	if err := rebootCmd.Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}

func maintainRebootRequiredMetric(nodeID string) {
	for {
		if sentinelExists() {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(1)
		} else {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(0)
		}
		time.Sleep(time.Minute)
	}
}

// nodeMeta is used to remember information across reboots
type nodeMeta struct {
	Unschedulable bool `json:"unschedulable"`
}

func rebootAsRequired(nodeID string, window *timewindow.TimeWindow, TTL time.Duration) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	lock := daemonsetlock.New(client, nodeID, dsNamespace, dsName, lockAnnotation)

	nodeMeta := nodeMeta{}
	if holding(lock, &nodeMeta) {
		if !nodeMeta.Unschedulable {
			node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
			if err != nil {
				log.Fatal(err)
			}
			uncordon(client, node)
		}
		release(lock)
	}

	preferNoScheduleTaint := taints.New(client, nodeID, preferNoScheduleTaintName, v1.TaintEffectPreferNoSchedule)

	// Remove taint immediately during startup to quickly allow scheduling again.
	if !rebootRequired() {
		preferNoScheduleTaint.Disable()
	}

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, period)
	for range tick {
		if !window.Contains(time.Now()) {
			// Remove taint outside the reboot time window to allow for normal operation.
			preferNoScheduleTaint.Disable()
			continue
		}

		if !rebootRequired() {
			preferNoScheduleTaint.Disable()
			continue
		}

		if rebootBlocked(client, nodeID) {
			continue
		}

		node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
		if err != nil {
			log.Fatal(err)
		}
		nodeMeta.Unschedulable = node.Spec.Unschedulable

		if !acquire(lock, &nodeMeta, TTL) {
			// Prefer to not schedule pods onto this node to avoid draing the same pod multiple times.
			preferNoScheduleTaint.Enable()
			continue
		}

		if !nodeMeta.Unschedulable {
			drain(client, node)
		}
		commandReboot(nodeID)
		for {
			log.Infof("Waiting for reboot")
			time.Sleep(time.Minute)
		}
	}
}

func root(cmd *cobra.Command, args []string) {
	log.Infof("Kubernetes Reboot Daemon: %s", version)

	nodeID := os.Getenv("KURED_NODE_ID")
	if nodeID == "" {
		log.Fatal("KURED_NODE_ID environment variable required")
	}

	window, err := timewindow.New(rebootDays, rebootStart, rebootEnd, timezone)
	if err != nil {
		log.Fatalf("Failed to build time window: %v", err)
	}

	log.Infof("Node ID: %s", nodeID)
	log.Infof("Lock Annotation: %s/%s:%s", dsNamespace, dsName, lockAnnotation)
	if lockTTL > 0 {
		log.Infof("Lock TTL set, lock will expire after: %v", lockTTL)
	} else {
		log.Info("Lock TTL not set, lock will remain until being released")
	}
	log.Infof("PreferNoSchedule taint: %s", preferNoScheduleTaintName)
	log.Infof("Reboot Sentinel: %s every %v", rebootSentinel, period)
	log.Infof("Blocking Pod Selectors: %v", podSelectors)
	log.Infof("Reboot on: %v", window)

	go rebootAsRequired(nodeID, window, lockTTL)
	go maintainRebootRequiredMetric(nodeID)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
