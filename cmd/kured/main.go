package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"time"

	papi "github.com/prometheus/client_golang/api"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubectldrain "k8s.io/kubectl/pkg/drain"

	"github.com/google/shlex"

	shoutrrr "github.com/containrrr/shoutrrr"
	"github.com/kubereboot/kured/pkg/alerts"
	"github.com/kubereboot/kured/pkg/daemonsetlock"
	"github.com/kubereboot/kured/pkg/delaytick"
	"github.com/kubereboot/kured/pkg/reboot"
	"github.com/kubereboot/kured/pkg/taints"
	"github.com/kubereboot/kured/pkg/timewindow"
	"github.com/kubereboot/kured/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version = "unreleased"

	// Command line flags
	forceReboot                     bool
	drainDelay                      time.Duration
	drainTimeout                    time.Duration
	rebootDelay                     time.Duration
	rebootMethod                    string
	period                          time.Duration
	metricsHost                     string
	metricsPort                     int
	drainGracePeriod                int
	drainPodSelector                string
	skipWaitForDeleteTimeoutSeconds int
	dsNamespace                     string
	dsName                          string
	lockAnnotation                  string
	lockTTL                         time.Duration
	lockReleaseDelay                time.Duration
	prometheusURL                   string
	preferNoScheduleTaintName       string
	alertFilter                     *regexp.Regexp
	alertFilterMatchOnly            bool
	alertFiringOnly                 bool
	rebootSentinelFile              string
	rebootSentinelCommand           string
	notifyURL                       string
	slackHookURL                    string
	slackUsername                   string
	slackChannel                    string
	messageTemplateDrain            string
	messageTemplateReboot           string
	messageTemplateUncordon         string
	podSelectors                    []string
	rebootCommand                   string
	rebootSignal                    int
	logFormat                       string
	preRebootNodeLabels             []string
	postRebootNodeLabels            []string
	nodeID                          string
	concurrency                     int

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
	// EnvPrefix The environment variable prefix of all environment variables bound to our command line flags.
	EnvPrefix = "KURED"

	// MethodCommand is used as "--reboot-method" value when rebooting with the configured "--reboot-command"
	MethodCommand = "command"
	// MethodSignal is used as "--reboot-method" value when rebooting with a SIGRTMIN+5 signal.
	MethodSignal = "signal"

	sigTrminPlus5 = 34 + 5
)

func init() {
	prometheus.MustRegister(rebootRequiredGauge)
}

func main() {
	cmd := NewRootCommand()

	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// NewRootCommand construct the Cobra root command
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:               "kured",
		Short:             "Kubernetes Reboot Daemon",
		PersistentPreRunE: bindViper,
		PreRun:            flagCheck,
		Run:               root}

	rootCmd.PersistentFlags().StringVar(&nodeID, "node-id", "",
		"node name kured runs on, should be passed down from spec.nodeName via KURED_NODE_ID environment variable")
	rootCmd.PersistentFlags().BoolVar(&forceReboot, "force-reboot", false,
		"force a reboot even if the drain fails or times out")
	rootCmd.PersistentFlags().StringVar(&metricsHost, "metrics-host", "",
		"host where metrics will listen")
	rootCmd.PersistentFlags().IntVar(&metricsPort, "metrics-port", 8080,
		"port number where metrics will listen")
	rootCmd.PersistentFlags().IntVar(&drainGracePeriod, "drain-grace-period", -1,
		"time in seconds given to each pod to terminate gracefully, if negative, the default value specified in the pod will be used")
	rootCmd.PersistentFlags().StringVar(&drainPodSelector, "drain-pod-selector", "",
		"only drain pods with labels matching the selector (default: '', all pods)")
	rootCmd.PersistentFlags().IntVar(&skipWaitForDeleteTimeoutSeconds, "skip-wait-for-delete-timeout", 0,
		"when seconds is greater than zero, skip waiting for the pods whose deletion timestamp is older than N seconds while draining a node")
	rootCmd.PersistentFlags().DurationVar(&drainDelay, "drain-delay", 0,
		"delay drain for this duration (default: 0, disabled)")
	rootCmd.PersistentFlags().DurationVar(&drainTimeout, "drain-timeout", 0,
		"timeout after which the drain is aborted (default: 0, infinite time)")
	rootCmd.PersistentFlags().DurationVar(&rebootDelay, "reboot-delay", 0,
		"delay reboot for this duration (default: 0, disabled)")
	rootCmd.PersistentFlags().StringVar(&rebootMethod, "reboot-method", "command",
		"method to use for reboots. Available: command")
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
	rootCmd.PersistentFlags().BoolVar(&alertFilterMatchOnly, "alert-filter-match-only", false,
		"Only block if the alert-filter-regexp matches active alerts")
	rootCmd.PersistentFlags().BoolVar(&alertFiringOnly, "alert-firing-only", false,
		"only consider firing alerts when checking for active alerts")
	rootCmd.PersistentFlags().StringVar(&rebootSentinelFile, "reboot-sentinel", "/var/run/reboot-required",
		"path to file whose existence triggers the reboot command")
	rootCmd.PersistentFlags().StringVar(&preferNoScheduleTaintName, "prefer-no-schedule-taint", "",
		"Taint name applied during pending node reboot (to prevent receiving additional pods from other rebooting nodes). Disabled by default. Set e.g. to \"weave.works/kured-node-reboot\" to enable tainting.")
	rootCmd.PersistentFlags().StringVar(&rebootSentinelCommand, "reboot-sentinel-command", "",
		"command for which a zero return code will trigger a reboot command")
	rootCmd.PersistentFlags().StringVar(&rebootCommand, "reboot-command", "/bin/systemctl reboot",
		"command to run when a reboot is required")
	rootCmd.PersistentFlags().IntVar(&concurrency, "concurrency", 1,
		"amount of nodes to concurrently reboot. Defaults to 1")
	rootCmd.PersistentFlags().IntVar(&rebootSignal, "reboot-signal", sigTrminPlus5,
		"signal to use for reboot, SIGRTMIN+5 by default.")

	rootCmd.PersistentFlags().StringVar(&slackHookURL, "slack-hook-url", "",
		"slack hook URL for reboot notifications [deprecated in favor of --notify-url]")
	rootCmd.PersistentFlags().StringVar(&slackUsername, "slack-username", "kured",
		"slack username for reboot notifications")
	rootCmd.PersistentFlags().StringVar(&slackChannel, "slack-channel", "",
		"slack channel for reboot notifications")
	rootCmd.PersistentFlags().StringVar(&notifyURL, "notify-url", "",
		"notify URL for reboot notifications (cannot use with --slack-hook-url flags)")
	rootCmd.PersistentFlags().StringVar(&messageTemplateUncordon, "message-template-uncordon", "Node %s rebooted & uncordoned successfully!",
		"message template used to notify about a node being successfully uncordoned")
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

	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text",
		"use text or json log format")

	rootCmd.PersistentFlags().StringSliceVar(&preRebootNodeLabels, "pre-reboot-node-labels", nil,
		"labels to add to nodes before cordoning")
	rootCmd.PersistentFlags().StringSliceVar(&postRebootNodeLabels, "post-reboot-node-labels", nil,
		"labels to add to nodes after uncordoning")

	return rootCmd
}

// func that checks for deprecated slack-notification-related flags and node labels that do not match
func flagCheck(cmd *cobra.Command, args []string) {
	if slackHookURL != "" && notifyURL != "" {
		log.Warnf("Cannot use both --notify-url and --slack-hook-url flags. Kured will use --notify-url flag only...")
	}
	if notifyURL != "" {
		notifyURL = stripQuotes(notifyURL)
	} else if slackHookURL != "" {
		slackHookURL = stripQuotes(slackHookURL)
		log.Warnf("Deprecated flag(s). Please use --notify-url flag instead.")
		trataURL, err := url.Parse(slackHookURL)
		if err != nil {
			log.Warnf("slack-hook-url is not properly formatted... no notification will be sent: %v\n", err)
		}
		if len(strings.Split(strings.Trim(trataURL.Path, "/services/"), "/")) != 3 {
			log.Warnf("slack-hook-url is not properly formatted... no notification will be sent: unexpected number of / in URL\n")
		} else {
			notifyURL = fmt.Sprintf("slack://%s", strings.Trim(trataURL.Path, "/services/"))
		}
	}
	var preRebootNodeLabelKeys, postRebootNodeLabelKeys []string
	for _, label := range preRebootNodeLabels {
		preRebootNodeLabelKeys = append(preRebootNodeLabelKeys, strings.Split(label, "=")[0])
	}
	for _, label := range postRebootNodeLabels {
		postRebootNodeLabelKeys = append(postRebootNodeLabelKeys, strings.Split(label, "=")[0])
	}
	sort.Strings(preRebootNodeLabelKeys)
	sort.Strings(postRebootNodeLabelKeys)
	if !reflect.DeepEqual(preRebootNodeLabelKeys, postRebootNodeLabelKeys) {
		log.Warnf("pre-reboot-node-labels keys and post-reboot-node-labels keys do not match. This may result in unexpected behaviour.")
	}
}

// stripQuotes removes any literal single or double quote chars that surround a string
func stripQuotes(str string) string {
	if len(str) > 2 {
		firstChar := str[0]
		lastChar := str[len(str)-1]
		if firstChar == lastChar && (firstChar == '"' || firstChar == '\'') {
			return str[1 : len(str)-1]
		}
	}
	// return the original string if it has a length of zero or one
	return str
}

// bindViper initializes viper and binds command flags with environment variables
func bindViper(cmd *cobra.Command, args []string) error {
	v := viper.New()

	v.SetEnvPrefix(EnvPrefix)
	v.AutomaticEnv()
	bindFlags(cmd, v)

	return nil
}

// bindFlags binds each cobra flag to its associated viper configuration (environment variable)
func bindFlags(cmd *cobra.Command, v *viper.Viper) {
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		// Environment variables can't have dashes in them, so bind them to their equivalent keys with underscores
		if strings.Contains(f.Name, "-") {
			v.BindEnv(f.Name, flagToEnvVar(f.Name))
		}

		// Apply the viper config value to the flag when the flag is not set and viper has a value
		if !f.Changed && v.IsSet(f.Name) {
			val := v.Get(f.Name)
			log.Infof("Binding %s command flag to environment variable: %s", f.Name, flagToEnvVar(f.Name))
			cmd.Flags().Set(f.Name, fmt.Sprintf("%v", val))
		}
	})
}

// flagToEnvVar converts command flag name to equivalent environment variable name
func flagToEnvVar(flag string) string {
	envVarSuffix := strings.ToUpper(strings.ReplaceAll(flag, "-", "_"))
	return fmt.Sprintf("%s_%s", EnvPrefix, envVarSuffix)
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
	cmd := util.NewCommand(sentinelCommand[0], sentinelCommand[1:]...)
	if err := cmd.Run(); err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
			// We assume a non-zero exit code means 'reboot not required', but of course
			// the user could have misconfigured the sentinel command or something else
			// went wrong during its execution. In that case, not entering a reboot loop
			// is the right thing to do, and we are logging stdout/stderr of the command
			// so it should be obvious what is wrong.
			if cmd.ProcessState.ExitCode() != 1 {
				log.Warnf("sentinel command ended with unexpected exit code: %v", cmd.ProcessState.ExitCode())
			}
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
	// prometheusClient to make prometheus-go-client and api config available
	// into the PrometheusBlockingChecker struct
	promClient *alerts.PromClient
	// regexp used to get alerts
	filter *regexp.Regexp
	// bool to indicate if only firing alerts should be considered
	firingOnly bool
	// bool to indicate that we're only blocking on alerts which match the filter
	filterMatchOnly bool
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
	alertNames, err := pb.promClient.ActiveAlerts(pb.filter, pb.firingOnly, pb.filterMatchOnly)
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
	fieldSelector := fmt.Sprintf("spec.nodeName=%s,status.phase!=Succeeded,status.phase!=Failed,status.phase!=Unknown", kb.nodename)
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

func holding(lock *daemonsetlock.DaemonSetLock, metadata interface{}, isMultiLock bool) bool {
	var holding bool
	var err error
	if isMultiLock {
		holding, err = lock.TestMultiple()
	} else {
		holding, err = lock.Test(metadata)
	}
	if err != nil {
		log.Fatalf("Error testing lock: %v", err)
	}
	if holding {
		log.Infof("Holding lock")
	}
	return holding
}

func acquire(lock *daemonsetlock.DaemonSetLock, metadata interface{}, TTL time.Duration, maxOwners int) bool {
	var holding bool
	var holder string
	var err error
	if maxOwners > 1 {
		var holders []string
		holding, holders, err = lock.AcquireMultiple(metadata, TTL, maxOwners)
		holder = strings.Join(holders, ",")
	} else {
		holding, holder, err = lock.Acquire(metadata, TTL)
	}
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

func release(lock *daemonsetlock.DaemonSetLock, isMultiLock bool) {
	log.Infof("Releasing lock")

	var err error
	if isMultiLock {
		err = lock.ReleaseMultiple()
	} else {
		err = lock.Release()
	}
	if err != nil {
		log.Fatalf("Error releasing lock: %v", err)
	}
}

func drain(client *kubernetes.Clientset, node *v1.Node) error {
	nodename := node.GetName()

	if preRebootNodeLabels != nil {
		updateNodeLabels(client, node, preRebootNodeLabels)
	}

	if drainDelay > 0 {
		log.Infof("Delaying drain for %v", drainDelay)
		time.Sleep(drainDelay)
	}

	log.Infof("Draining node %s", nodename)

	if notifyURL != "" {
		if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateDrain, nodename)); err != nil {
			log.Warnf("Error notifying: %v", err)
		}
	}

	drainer := &kubectldrain.Helper{
		Client:                          client,
		Ctx:                             context.Background(),
		GracePeriodSeconds:              drainGracePeriod,
		PodSelector:                     drainPodSelector,
		SkipWaitForDeleteTimeoutSeconds: skipWaitForDeleteTimeoutSeconds,
		Force:                           true,
		DeleteEmptyDirData:              true,
		IgnoreAllDaemonSets:             true,
		ErrOut:                          os.Stderr,
		Out:                             os.Stdout,
		Timeout:                         drainTimeout,
	}

	if err := kubectldrain.RunCordonOrUncordon(drainer, node, true); err != nil {
		log.Errorf("Error cordonning %s: %v", nodename, err)
		return err
	}

	if err := kubectldrain.RunNodeDrain(drainer, nodename); err != nil {
		log.Errorf("Error draining %s: %v", nodename, err)
		return err
	}
	return nil
}

func uncordon(client *kubernetes.Clientset, node *v1.Node) error {
	nodename := node.GetName()
	log.Infof("Uncordoning node %s", nodename)
	drainer := &kubectldrain.Helper{
		Client: client,
		ErrOut: os.Stderr,
		Out:    os.Stdout,
		Ctx:    context.Background(),
	}
	if err := kubectldrain.RunCordonOrUncordon(drainer, node, false); err != nil {
		log.Fatalf("Error uncordonning %s: %v", nodename, err)
		return err
	} else if postRebootNodeLabels != nil {
		updateNodeLabels(client, node, postRebootNodeLabels)
	}
	return nil
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

func addNodeAnnotations(client *kubernetes.Clientset, nodeID string, annotations map[string]string) error {
	node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
	if err != nil {
		log.Errorf("Error retrieving node object via k8s API: %s", err)
		return err
	}
	for k, v := range annotations {
		node.Annotations[k] = v
		log.Infof("Adding node %s annotation: %s=%s", node.GetName(), k, v)
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		log.Errorf("Error marshalling node object into JSON: %v", err)
		return err
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), node.GetName(), types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		var annotationsErr string
		for k, v := range annotations {
			annotationsErr += fmt.Sprintf("%s=%s ", k, v)
		}
		log.Errorf("Error adding node annotations %s via k8s API: %v", annotationsErr, err)
		return err
	}
	return nil
}

func deleteNodeAnnotation(client *kubernetes.Clientset, nodeID, key string) error {
	log.Infof("Deleting node %s annotation %s", nodeID, key)

	// JSON Patch takes as path input a JSON Pointer, defined in RFC6901
	// So we replace all instances of "/" with "~1" as per:
	// https://tools.ietf.org/html/rfc6901#section-3
	patch := []byte(fmt.Sprintf("[{\"op\":\"remove\",\"path\":\"/metadata/annotations/%s\"}]", strings.ReplaceAll(key, "/", "~1")))
	_, err := client.CoreV1().Nodes().Patch(context.TODO(), nodeID, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		log.Errorf("Error deleting node annotation %s via k8s API: %v", key, err)
		return err
	}
	return nil
}

func updateNodeLabels(client *kubernetes.Clientset, node *v1.Node, labels []string) {
	labelsMap := make(map[string]string)
	for _, label := range labels {
		k := strings.Split(label, "=")[0]
		v := strings.Split(label, "=")[1]
		labelsMap[k] = v
		log.Infof("Updating node %s label: %s=%s", node.GetName(), k, v)
	}

	bytes, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labelsMap,
		},
	})
	if err != nil {
		log.Fatalf("Error marshalling node object into JSON: %v", err)
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), node.GetName(), types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		var labelsErr string
		for _, label := range labels {
			k := strings.Split(label, "=")[0]
			v := strings.Split(label, "=")[1]
			labelsErr += fmt.Sprintf("%s=%s ", k, v)
		}
		log.Errorf("Error updating node labels %s via k8s API: %v", labelsErr, err)
	}
}

func rebootAsRequired(nodeID string, booter reboot.Reboot, sentinelCommand []string, window *timewindow.TimeWindow, TTL time.Duration, releaseDelay time.Duration) {
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
	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, 1*time.Minute)
	for range tick {
		if holding(lock, &nodeMeta, concurrency > 1) {
			node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
			if err != nil {
				log.Errorf("Error retrieving node object via k8s API: %v", err)
				continue
			}
			if !nodeMeta.Unschedulable {
				err = uncordon(client, node)
				if err != nil {
					log.Errorf("Unable to uncordon %s: %v, will continue to hold lock and retry uncordon", node.GetName(), err)
					continue
				} else {
					if notifyURL != "" {
						if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateUncordon, nodeID)); err != nil {
							log.Warnf("Error notifying: %v", err)
						}
					}
				}
			}
			// If we're holding the lock we know we've tried, in a prior run, to reboot
			// So (1) we want to confirm that the reboot succeeded practically ( !rebootRequired() )
			// And (2) check if we previously annotated the node that it was in the process of being rebooted,
			// And finally (3) if it has that annotation, to delete it.
			// This indicates to other node tools running on the cluster that this node may be a candidate for maintenance
			if annotateNodes && !rebootRequired(sentinelCommand) {
				if _, ok := node.Annotations[KuredRebootInProgressAnnotation]; ok {
					err := deleteNodeAnnotation(client, nodeID, KuredRebootInProgressAnnotation)
					if err != nil {
						continue
					}
				}
			}
			throttle(releaseDelay)
			release(lock, concurrency > 1)
			break
		} else {
			break
		}
	}

	preferNoScheduleTaint := taints.New(client, nodeID, preferNoScheduleTaintName, v1.TaintEffectPreferNoSchedule)

	// Remove taint immediately during startup to quickly allow scheduling again.
	if !rebootRequired(sentinelCommand) {
		preferNoScheduleTaint.Disable()
	}

	// instantiate prometheus client
	promClient, err := alerts.NewPromClient(papi.Config{Address: prometheusURL})
	if err != nil {
		log.Fatal("Unable to create prometheus client: ", err)
	}

	source = rand.NewSource(time.Now().UnixNano())
	tick = delaytick.New(source, period)
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
				err := addNodeAnnotations(client, nodeID, annotations)
				if err != nil {
					continue
				}
			}
		}

		var blockCheckers []RebootBlocker
		if prometheusURL != "" {
			blockCheckers = append(blockCheckers, PrometheusBlockingChecker{promClient: promClient, filter: alertFilter, firingOnly: alertFiringOnly, filterMatchOnly: alertFilterMatchOnly})
		}
		if podSelectors != nil {
			blockCheckers = append(blockCheckers, KubernetesBlockingChecker{client: client, nodename: nodeID, filter: podSelectors})
		}

		var rebootRequiredBlockCondition string
		if rebootBlocked(blockCheckers...) {
			rebootRequiredBlockCondition = ", but blocked at this time"
			continue
		}
		log.Infof("Reboot required%s", rebootRequiredBlockCondition)

		if !holding(lock, &nodeMeta, concurrency > 1) && !acquire(lock, &nodeMeta, TTL, concurrency) {
			// Prefer to not schedule pods onto this node to avoid draing the same pod multiple times.
			preferNoScheduleTaint.Enable()
			continue
		}

		err = drain(client, node)
		if err != nil {
			if !forceReboot {
				log.Errorf("Unable to cordon or drain %s: %v, will release lock and retry cordon and drain before rebooting when lock is next acquired", node.GetName(), err)
				release(lock, concurrency > 1)
				log.Infof("Performing a best-effort uncordon after failed cordon and drain")
				uncordon(client, node)
				continue
			}
		}

		if rebootDelay > 0 {
			log.Infof("Delaying reboot for %v", rebootDelay)
			time.Sleep(rebootDelay)
		}

		if notifyURL != "" {
			if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateReboot, nodeID)); err != nil {
				log.Warnf("Error notifying: %v", err)
			}
		}

		booter.Reboot()
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
	if logFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	log.Infof("Kubernetes Reboot Daemon: %s", version)

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
	log.Infof("Concurrency: %v", concurrency)
	log.Infof("Reboot method: %s", rebootMethod)
	if rebootCommand == MethodCommand {
		log.Infof("Reboot command: %s", restartCommand)
	} else {
		log.Infof("Reboot signal: %v", rebootSignal)
	}

	if annotateNodes {
		log.Infof("Will annotate nodes during kured reboot operations")
	}

	// To run those commands as it was the host, we'll use nsenter
	// Relies on hostPID:true and privileged:true to enter host mount space
	// PID set to 1, until we have a better discovery mechanism.
	hostRestartCommand := buildHostCommand(1, restartCommand)

	// Only wrap sentinel-command with nsenter, if a custom-command was configured, otherwise use the host-path mount
	hostSentinelCommand := sentinelCommand
	if rebootSentinelCommand != "" {
		hostSentinelCommand = buildHostCommand(1, sentinelCommand)
	}

	var booter reboot.Reboot
	if rebootMethod == MethodCommand {
		booter = reboot.NewCommandReboot(nodeID, hostRestartCommand)
	} else if rebootMethod == MethodSignal {
		booter = reboot.NewSignalReboot(nodeID, rebootSignal)
	} else {
		log.Fatalf("Invalid reboot-method configured: %s", rebootMethod)
	}

	go rebootAsRequired(nodeID, booter, hostSentinelCommand, window, lockTTL, lockReleaseDelay)
	go maintainRebootRequiredMetric(nodeID, hostSentinelCommand)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", metricsHost, metricsPort), nil))
}
