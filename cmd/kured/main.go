package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubectldrain "k8s.io/kubectl/pkg/drain"

	"github.com/google/shlex"

	shoutrrr "github.com/containrrr/shoutrrr"
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
	period                    time.Duration
	dsNamespace               string
	dsName                    string
	lockAnnotation            string
	lockTTL                   time.Duration
	lockReleaseDelay          time.Duration
	prometheusURL             string
	preferNoScheduleTaintName string
	alertFilter               *regexp.Regexp
	rebootSentinelFile        string
	rebootSentinelCommand     string
	notifyURL                 string
	slackHookURL              string
	slackUsername             string
	slackChannel              string
	messageTemplateDrain      string
	messageTemplateReboot     string
	podSelectors              []string
	rebootCommand             string

	rebootDays    []string
	rebootStart   string
	rebootEnd     string
	timezone      string
	annotateNodes bool

	// Metrics
	rebootRequiredGauge = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Subsystem: "kured",
		Name:      "reboot_required",
		Help:      "OS requires reboot due to software updates.",
	}, []string{"node"})
)

const (
	// KuredNodeLockAnnotation is the canonical string value for the kured node-lock annotation
	KuredNodeLockAnnotation string = "weave.works/kured-node-lock"
	// KuredRebootInProgressAnnotation is the canonical string value for the kured reboot-in-progress annotation
	KuredRebootInProgressAnnotation string = "weave.works/kured-reboot-in-progress"
	// KuredMostRecentRebootNeededAnnotation is the canonical string value for the kured most-recent-reboot-needed annotation
	KuredMostRecentRebootNeededAnnotation string = "weave.works/kured-most-recent-reboot-needed"
)

func init() {
	prometheus.MustRegister(rebootRequiredGauge)
}

func main() {
	rootCmd := &cobra.Command{
		Use:    "kured",
		Short:  "Kubernetes Reboot Daemon",
		PreRun: flagCheck,
		Run:    root}

	rootCmd.PersistentFlags().DurationVar(&period, "period", time.Minute*60,
		"sentinel check period")
	rootCmd.PersistentFlags().StringVar(&dsNamespace, "ds-namespace", "kube-system",
		"namespace containing daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&dsName, "ds-name", "kured",
		"name of daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&lockAnnotation, "lock-annotation", KuredNodeLockAnnotation,
		"annotation in which to record locking node")
	rootCmd.PersistentFlags().DurationVar(&lockTTL, "lock-ttl", 0,
		"expire lock annotation after this duration (default: 0, disabled)")
	rootCmd.PersistentFlags().DurationVar(&lockReleaseDelay, "lock-release-delay", 0,
		"delay lock release for this duration (default: 0, disabled)")
	rootCmd.PersistentFlags().StringVar(&prometheusURL, "prometheus-url", "",
		"Prometheus instance to probe for active alerts")
	rootCmd.PersistentFlags().Var(&regexpValue{&alertFilter}, "alert-filter-regexp",
		"alert names to ignore when checking for active alerts")
	rootCmd.PersistentFlags().StringVar(&rebootSentinelFile, "reboot-sentinel", "/var/run/reboot-required",
		"path to file whose existence triggers the reboot command")
	rootCmd.PersistentFlags().StringVar(&preferNoScheduleTaintName, "prefer-no-schedule-taint", "",
		"Taint name applied during pending node reboot (to prevent receiving additional pods from other rebooting nodes). Disabled by default. Set e.g. to \"weave.works/kured-node-reboot\" to enable tainting.")
	rootCmd.PersistentFlags().StringVar(&rebootSentinelCommand, "reboot-sentinel-command", "",
		"command for which a zero return code will trigger a reboot command")
	rootCmd.PersistentFlags().StringVar(&rebootCommand, "reboot-command", "/bin/systemctl reboot",
		"command to run when a reboot is required")

	rootCmd.PersistentFlags().StringVar(&slackHookURL, "slack-hook-url", "",
		"slack hook URL for notifications")
	rootCmd.PersistentFlags().StringVar(&slackUsername, "slack-username", "kured",
		"slack username for notifications")
	rootCmd.PersistentFlags().StringVar(&slackChannel, "slack-channel", "",
		"slack channel for reboot notfications")
	rootCmd.PersistentFlags().StringVar(&notifyURL, "notify-url", "",
		"notify URL for reboot notfications")
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

	rootCmd.PersistentFlags().BoolVar(&annotateNodes, "annotate-nodes", false,
		"if set, the annotations 'weave.works/kured-reboot-in-progress' and 'weave.works/kured-most-recent-reboot-needed' will be given to nodes undergoing kured reboots")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// temporary func that checks for deprecated slack-notification-related flags
func flagCheck(cmd *cobra.Command, args []string) {
	if slackHookURL != "" && notifyURL != "" {
		log.Warnf("Cannot use both --notify-url and --slack-hook-url flags. Kured will use --notify-url flag only...")
		slackHookURL = ""
	}
	if slackChannel != "" || slackHookURL != "" || slackUsername != "" {
		log.Warnf("Deprecated flag(s). Please use --notify-url flag instead.")
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

// buildHostCommand writes a new command to run in the host namespace
// Rancher based need different pid
func buildHostCommand(pid int, command []string) []string {

	// From the container, we nsenter into the proper PID to run the hostCommand.
	// For this, kured daemonset need to be configured with hostPID:true and privileged:true
	cmd := []string{"/usr/bin/nsenter", fmt.Sprintf("-m/proc/%d/ns/mnt", pid), "--"}
	cmd = append(cmd, command...)
	return cmd
}

func rebootRequired(sentinelCommand []string) bool {
	if err := newCommand(sentinelCommand[0], sentinelCommand[1:]...).Run(); err != nil {
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

// RebootBlocker interface should be implemented by types
// to know if their instantiations should block a reboot
type RebootBlocker interface {
	isBlocked() bool
}

// PrometheusBlockingChecker contains info for connecting
// to prometheus, and can give info about whether a reboot should be blocked
type PrometheusBlockingChecker struct {
	// URL to contact prometheus API for checking alerts
	promURL string
	// regexp used to get alerts
	filter *regexp.Regexp
}

// KubernetesBlockingChecker contains info for connecting
// to k8s, and can give info about whether a reboot should be blocked
type KubernetesBlockingChecker struct {
	// client used to contact kubernetes API
	client   *kubernetes.Clientset
	nodename string
	// lised used to filter pods (podSelector)
	filter []string
}

func (pb PrometheusBlockingChecker) isBlocked() bool {
	alertNames, err := alerts.PrometheusActiveAlerts(pb.promURL, pb.filter)
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

func (kb KubernetesBlockingChecker) isBlocked() bool {
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", kb.nodename)
	for _, labelSelector := range kb.filter {
		podList, err := kb.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
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

func rebootBlocked(blockers ...RebootBlocker) bool {
	for _, blocker := range blockers {
		if blocker.isBlocked() {
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
func throttle(releaseDelay time.Duration) {
	if releaseDelay > 0 {
		log.Infof("Delaying lock release by %v", releaseDelay)
		time.Sleep(releaseDelay)
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
	if notifyURL != "" {
		if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateDrain, nodename)); err != nil {
			log.Warnf("Error notifying: %v", err)
		}
	}

	drainer := &kubectldrain.Helper{
		Client:              client,
		GracePeriodSeconds:  -1,
		Force:               true,
		DeleteEmptyDirData:  true,
		IgnoreAllDaemonSets: true,
		ErrOut:              os.Stderr,
		Out:                 os.Stdout,
	}
	if err := kubectldrain.RunCordonOrUncordon(drainer, node, true); err != nil {
		log.Fatalf("Error cordonning %s: %v", nodename, err)
	}

	if err := kubectldrain.RunNodeDrain(drainer, nodename); err != nil {
		log.Fatalf("Error draining %s: %v", nodename, err)
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

func invokeReboot(nodeID string, rebootCommand []string) {
	log.Infof("Running command: %s for node: %s", rebootCommand, nodeID)

	if slackHookURL != "" {
		if err := slack.NotifyReboot(slackHookURL, slackUsername, slackChannel, messageTemplateReboot, nodeID); err != nil {
			log.Warnf("Error notifying slack: %v", err)
		}
	}

	if notifyURL != "" {
		if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateReboot, nodeID)); err != nil {
			log.Warnf("Error notifying: %v", err)
		}
	}

	if err := newCommand(rebootCommand[0], rebootCommand[1:]...).Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}

func maintainRebootRequiredMetric(nodeID string, sentinelCommand []string) {
	for {
		if rebootRequired(sentinelCommand) {
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

func addNodeAnnotations(client *kubernetes.Clientset, nodeID string, annotations map[string]string) {
	node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
	if err != nil {
		log.Fatalf("Error retrieving node object via k8s API: %s", err)
	}
	for k, v := range annotations {
		node.Annotations[k] = v
		log.Infof("Adding node %s annotation: %s=%s", node.GetName(), k, v)
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		log.Fatalf("Error marshalling node object into JSON: %v", err)
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), node.GetName(), types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		var annotationsErr string
		for k, v := range annotations {
			annotationsErr += fmt.Sprintf("%s=%s ", k, v)
		}
		log.Fatalf("Error adding node annotations %s via k8s API: %v", annotationsErr, err)
	}
}

func deleteNodeAnnotation(client *kubernetes.Clientset, nodeID, key string) {
	log.Infof("Deleting node %s annotation %s", nodeID, key)

	// JSON Patch takes as path input a JSON Pointer, defined in RFC6901
	// So we replace all instances of "/" with "~1" as per:
	// https://tools.ietf.org/html/rfc6901#section-3
	patch := []byte(fmt.Sprintf("[{\"op\":\"remove\",\"path\":\"/metadata/annotations/%s\"}]", strings.ReplaceAll(key, "/", "~1")))
	_, err := client.CoreV1().Nodes().Patch(context.TODO(), nodeID, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		log.Fatalf("Error deleting node annotation %s via k8s API: %v", key, err)
	}
}

func rebootAsRequired(nodeID string, rebootCommand []string, sentinelCommand []string, window *timewindow.TimeWindow, TTL time.Duration, releaseDelay time.Duration) {
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
		node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Error retrieving node object via k8s API: %v", err)
		}
		if !nodeMeta.Unschedulable {
			uncordon(client, node)
		}
		// If we're holding the lock we know we've tried, in a prior run, to reboot
		// So (1) we want to confirm that the reboot succeeded practically ( !rebootRequired() )
		// And (2) check if we previously annotated the node that it was in the process of being rebooted,
		// And finally (3) if it has that annotation, to delete it.
		// This indicates to other node tools running on the cluster that this node may be a candidate for maintenance
		if annotateNodes && !rebootRequired(sentinelCommand) {
			if _, ok := node.Annotations[KuredRebootInProgressAnnotation]; ok {
				deleteNodeAnnotation(client, nodeID, KuredRebootInProgressAnnotation)
			}
		}
		throttle(releaseDelay)
		release(lock)
	}

	preferNoScheduleTaint := taints.New(client, nodeID, preferNoScheduleTaintName, v1.TaintEffectPreferNoSchedule)

	// Remove taint immediately during startup to quickly allow scheduling again.
	if !rebootRequired(sentinelCommand) {
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

		if !rebootRequired(sentinelCommand) {
			log.Infof("Reboot not required")
			preferNoScheduleTaint.Disable()
			continue
		}
		log.Infof("Reboot required")

		var blockCheckers []RebootBlocker
		if prometheusURL != "" {
			blockCheckers = append(blockCheckers, PrometheusBlockingChecker{promURL: prometheusURL, filter: alertFilter})
		}
		if podSelectors != nil {
			blockCheckers = append(blockCheckers, KubernetesBlockingChecker{client: client, nodename: nodeID, filter: podSelectors})
		}

		if rebootBlocked(blockCheckers...) {
			continue
		}

		node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Error retrieving node object via k8s API: %v", err)
		}
		nodeMeta.Unschedulable = node.Spec.Unschedulable

		var timeNowString string
		if annotateNodes {
			if _, ok := node.Annotations[KuredRebootInProgressAnnotation]; !ok {
				timeNowString = time.Now().Format(time.RFC3339)
				// Annotate this node to indicate that "I am going to be rebooted!"
				// so that other node maintenance tools running on the cluster are aware that this node is in the process of a "state transition"
				annotations := map[string]string{KuredRebootInProgressAnnotation: timeNowString}
				// & annotate this node with a timestamp so that other node maintenance tools know how long it's been since this node has been marked for reboot
				annotations[KuredMostRecentRebootNeededAnnotation] = timeNowString
				addNodeAnnotations(client, nodeID, annotations)
			}
		}

		if !acquire(lock, &nodeMeta, TTL) {
			// Prefer to not schedule pods onto this node to avoid draing the same pod multiple times.
			preferNoScheduleTaint.Enable()
			continue
		}

		drain(client, node)
		invokeReboot(nodeID, rebootCommand)
		for {
			log.Infof("Waiting for reboot")
			time.Sleep(time.Minute)
		}
	}
}

// buildSentinelCommand creates the shell command line which will need wrapping to escape
// the container boundaries
func buildSentinelCommand(rebootSentinelFile string, rebootSentinelCommand string) []string {
	if rebootSentinelCommand != "" {
		cmd, err := shlex.Split(rebootSentinelCommand)
		if err != nil {
			log.Fatalf("Error parsing provided sentinel command: %v", err)
		}
		return cmd
	}
	return []string{"test", "-f", rebootSentinelFile}
}

// parseRebootCommand creates the shell command line which will need wrapping to escape
// the container boundaries
func parseRebootCommand(rebootCommand string) []string {
	command, err := shlex.Split(rebootCommand)
	if err != nil {
		log.Fatalf("Error parsing provided reboot command: %v", err)
	}
	return command
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

	sentinelCommand := buildSentinelCommand(rebootSentinelFile, rebootSentinelCommand)
	restartCommand := parseRebootCommand(rebootCommand)

	log.Infof("Node ID: %s", nodeID)
	log.Infof("Lock Annotation: %s/%s:%s", dsNamespace, dsName, lockAnnotation)
	if lockTTL > 0 {
		log.Infof("Lock TTL set, lock will expire after: %v", lockTTL)
	} else {
		log.Info("Lock TTL not set, lock will remain until being released")
	}
	if lockReleaseDelay > 0 {
		log.Infof("Lock release delay set, lock release will be delayed by: %v", lockReleaseDelay)
	} else {
		log.Info("Lock release delay not set, lock will be released immediately after rebooting")
	}
	log.Infof("PreferNoSchedule taint: %s", preferNoScheduleTaintName)
	log.Infof("Blocking Pod Selectors: %v", podSelectors)
	log.Infof("Reboot schedule: %v", window)
	log.Infof("Reboot check command: %s every %v", sentinelCommand, period)
	log.Infof("Reboot command: %s", restartCommand)
	if annotateNodes {
		log.Infof("Will annotate nodes during kured reboot operations")
	}

	// To run those commands as it was the host, we'll use nsenter
	// Relies on hostPID:true and privileged:true to enter host mount space
	// PID set to 1, until we have a better discovery mechanism.
	hostSentinelCommand := buildHostCommand(1, sentinelCommand)
	hostRestartCommand := buildHostCommand(1, restartCommand)

	go rebootAsRequired(nodeID, hostRestartCommand, hostSentinelCommand, window, lockTTL, lockReleaseDelay)
	go maintainRebootRequiredMetric(nodeID, hostSentinelCommand)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}
