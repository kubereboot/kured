// The main controller for kured
// This package is a reference implementation on how to reboot your nodes based on the different
// tools present in this project's modules
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/kubereboot/kured/internal/daemonsetlock"
	"github.com/kubereboot/kured/internal/taints"
	"github.com/kubereboot/kured/internal/timewindow"
	"github.com/kubereboot/kured/pkg/blockers"
	"github.com/kubereboot/kured/pkg/checkers"
	"github.com/kubereboot/kured/pkg/reboot"
	papi "github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	flag "github.com/spf13/pflag"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	kubectldrain "k8s.io/kubectl/pkg/drain"
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
	alertFilter                     regexpValue
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
	rebootBlockedCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: "kured",
		Name:      "reboot_blocked_reason",
		Help:      "Reboot required was blocked by event.",
	}, []string{"node", "reason"})
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

	sigTrminPlus5 = 34 + 5
)

func init() {
	prometheus.MustRegister(rebootRequiredGauge, rebootBlockedCounter)
}

func main() {

	flag.StringVar(&nodeID, "node-id", "",
		"node name kured runs on, should be passed down from spec.nodeName via KURED_NODE_ID environment variable")
	flag.BoolVar(&forceReboot, "force-reboot", false,
		"force a reboot even if the drain fails or times out")
	flag.StringVar(&metricsHost, "metrics-host", "",
		"host where metrics will listen")
	flag.IntVar(&metricsPort, "metrics-port", 8080,
		"port number where metrics will listen")
	flag.IntVar(&drainGracePeriod, "drain-grace-period", -1,
		"time in seconds given to each pod to terminate gracefully, if negative, the default value specified in the pod will be used")
	flag.StringVar(&drainPodSelector, "drain-pod-selector", "",
		"only drain pods with labels matching the selector (default: '', all pods)")
	flag.IntVar(&skipWaitForDeleteTimeoutSeconds, "skip-wait-for-delete-timeout", 0,
		"when seconds is greater than zero, skip waiting for the pods whose deletion timestamp is older than N seconds while draining a node")
	flag.DurationVar(&drainDelay, "drain-delay", 0,
		"delay drain for this duration (default: 0, disabled)")
	flag.DurationVar(&drainTimeout, "drain-timeout", 0,
		"timeout after which the drain is aborted (default: 0, infinite time)")
	flag.DurationVar(&rebootDelay, "reboot-delay", 0,
		"delay reboot for this duration (default: 0, disabled)")
	flag.StringVar(&rebootMethod, "reboot-method", "command",
		"method to use for reboots. Available: command")
	flag.DurationVar(&period, "period", time.Minute,
		"period at which the main operations are done")
	flag.StringVar(&dsNamespace, "ds-namespace", "kube-system",
		"namespace containing daemonset on which to place lock")
	flag.StringVar(&dsName, "ds-name", "kured",
		"name of daemonset on which to place lock")
	flag.StringVar(&lockAnnotation, "lock-annotation", KuredNodeLockAnnotation,
		"annotation in which to record locking node")
	flag.DurationVar(&lockTTL, "lock-ttl", 0,
		"expire lock annotation after this duration (default: 0, disabled)")
	flag.DurationVar(&lockReleaseDelay, "lock-release-delay", 0,
		"delay lock release for this duration (default: 0, disabled)")
	flag.StringVar(&prometheusURL, "prometheus-url", "",
		"Prometheus instance to probe for active alerts")
	flag.Var(&alertFilter, "alert-filter-regexp",
		"alert names to ignore when checking for active alerts")
	flag.BoolVar(&alertFilterMatchOnly, "alert-filter-match-only", false,
		"Only block if the alert-filter-regexp matches active alerts")
	flag.BoolVar(&alertFiringOnly, "alert-firing-only", false,
		"only consider firing alerts when checking for active alerts")
	flag.StringVar(&rebootSentinelFile, "reboot-sentinel", "/var/run/reboot-required",
		"path to file whose existence triggers the reboot command")
	flag.StringVar(&preferNoScheduleTaintName, "prefer-no-schedule-taint", "",
		"Taint name applied during pending node reboot (to prevent receiving additional pods from other rebooting nodes). Disabled by default. Set e.g. to \"weave.works/kured-node-reboot\" to enable tainting.")
	flag.StringVar(&rebootSentinelCommand, "reboot-sentinel-command", "",
		"command for which a zero return code will trigger a reboot command")
	flag.StringVar(&rebootCommand, "reboot-command", "/bin/systemctl reboot",
		"command to run when a reboot is required")
	flag.IntVar(&concurrency, "concurrency", 1,
		"amount of nodes to concurrently reboot. Defaults to 1")
	flag.IntVar(&rebootSignal, "reboot-signal", sigTrminPlus5,
		"signal to use for reboot, SIGRTMIN+5 by default.")
	flag.StringVar(&slackHookURL, "slack-hook-url", "",
		"slack hook URL for reboot notifications [deprecated in favor of --notify-url]")
	flag.StringVar(&slackUsername, "slack-username", "kured",
		"slack username for reboot notifications")
	flag.StringVar(&slackChannel, "slack-channel", "",
		"slack channel for reboot notifications")
	flag.StringVar(&notifyURL, "notify-url", "",
		"notify URL for reboot notifications (cannot use with --slack-hook-url flags)")
	flag.StringVar(&messageTemplateUncordon, "message-template-uncordon", "Node %s rebooted & uncordoned successfully!",
		"message template used to notify about a node being successfully uncordoned")
	flag.StringVar(&messageTemplateDrain, "message-template-drain", "Draining node %s",
		"message template used to notify about a node being drained")
	flag.StringVar(&messageTemplateReboot, "message-template-reboot", "Rebooting node %s",
		"message template used to notify about a node being rebooted")
	flag.StringArrayVar(&podSelectors, "blocking-pod-selector", nil,
		"label selector identifying pods whose presence should prevent reboots")
	flag.StringSliceVar(&rebootDays, "reboot-days", timewindow.EveryDay,
		"schedule reboot on these days")
	flag.StringVar(&rebootStart, "start-time", "0:00",
		"schedule reboot only after this time of day")
	flag.StringVar(&rebootEnd, "end-time", "23:59:59",
		"schedule reboot only before this time of day")
	flag.StringVar(&timezone, "time-zone", "UTC",
		"use this timezone for schedule inputs")
	flag.BoolVar(&annotateNodes, "annotate-nodes", false,
		"if set, the annotations 'weave.works/kured-reboot-in-progress' and 'weave.works/kured-most-recent-reboot-needed' will be given to nodes undergoing kured reboots")
	flag.StringVar(&logFormat, "log-format", "text",
		"use text or json log format")
	flag.StringSliceVar(&preRebootNodeLabels, "pre-reboot-node-labels", nil,
		"labels to add to nodes before cordoning")
	flag.StringSliceVar(&postRebootNodeLabels, "post-reboot-node-labels", nil,
		"labels to add to nodes after uncordoning")

	flag.Parse()

	// Load flags from environment variables
	LoadFromEnv()

	var logger *slog.Logger
	switch logFormat {
	case "json":
		logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	case "text":
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	default:
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
		logger.Info("incorrect configuration for logFormat, using text handler")
	}
	// For all the old calls using logger
	slog.SetDefault(logger)

	if nodeID == "" {
		slog.Error("KURED_NODE_ID environment variable required")
		os.Exit(1)
	}

	window, err := timewindow.New(rebootDays, rebootStart, rebootEnd, timezone)
	if err != nil {
		// TODO: Improve stacktrace with slog
		slog.Error(fmt.Sprintf("Failed to build time window: %v", err))
		os.Exit(2)
	}

	notifyURL = validateNotificationURL(notifyURL, slackHookURL)

	err = validateNodeLabels(preRebootNodeLabels, postRebootNodeLabels)
	if err != nil {
		slog.Info(err.Error(), "node", nodeID)
	}

	slog.Info("Starting Kubernetes Reboot Daemon",
		"version", version,
		"node", nodeID,
		"rebootPeriod", period,
		"concurrency", concurrency,
		"schedule", window,
		"method", rebootMethod,
	)

	slog.Info(fmt.Sprintf("preferNoSchedule taint: (%s)", preferNoScheduleTaintName), "node", nodeID)

	if annotateNodes {
		slog.Info("Will annotate nodes during kured reboot operations", "node", nodeID)
	}

	rebooter, err := reboot.NewRebooter(rebootMethod, rebootCommand, rebootSignal, rebootDelay, true, 1)
	if err != nil {
		slog.Error(fmt.Sprintf("unrecoverable error - failed to construct system rebooter: %v", err))
		os.Exit(3)
	}

	rebootChecker, err := checkers.NewRebootChecker(rebootSentinelCommand, rebootSentinelFile)
	if err != nil {
		slog.Error(fmt.Sprintf("unrecoverable error - failed to build reboot checker: %v", err))
		os.Exit(4)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		slog.Error(fmt.Sprintf("unrecoverable error - failed to load in cluster kubernetes config: %v", err))
		os.Exit(5)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		slog.Error(fmt.Sprintf("unrecoverable error - failed to load in cluster kubernetes config: %v", err))
		os.Exit(5)
	}

	var blockCheckers []blockers.RebootBlocker
	if prometheusURL != "" {
		slog.Info(fmt.Sprintf("Blocking reboot with prometheus alerts on %v", prometheusURL), "node", nodeID)
		blockCheckers = append(blockCheckers, blockers.NewPrometheusBlockingChecker(papi.Config{Address: prometheusURL}, alertFilter.Regexp, alertFiringOnly, alertFilterMatchOnly))
	}
	if podSelectors != nil {
		slog.Info(fmt.Sprintf("Blocking Pod Selectors: %v", podSelectors), "node", nodeID)
		blockCheckers = append(blockCheckers, blockers.NewKubernetesBlockingChecker(client, nodeID, podSelectors))
	}

	slog.Info(fmt.Sprintf("Lock Annotation: %s/%s:%s", dsNamespace, dsName, lockAnnotation), "node", nodeID)
	if lockTTL > 0 {
		slog.Info(fmt.Sprintf("Lock TTL set, lock will expire after: %v", lockTTL), "node", nodeID)
	} else {
		slog.Info("Lock TTL not set, lock will remain until being released", "node", nodeID)
	}

	if lockReleaseDelay > 0 {
		slog.Info(fmt.Sprintf("Lock release delay set, lock release will be delayed by: %v", lockReleaseDelay), "node", nodeID)
	} else {
		slog.Info("Lock release delay not set, lock will be released immediately after rebooting", "node", nodeID)
	}

	lock := daemonsetlock.New(client, nodeID, dsNamespace, dsName, lockAnnotation, lockTTL, concurrency, lockReleaseDelay)

	go rebootAsRequired(nodeID, rebooter, rebootChecker, blockCheckers, window, lock, client, period)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", metricsHost, metricsPort), nil)) // #nosec G114
}

func validateNodeLabels(preRebootNodeLabels []string, postRebootNodeLabels []string) error {
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
		return fmt.Errorf("pre-reboot-node-labels keys and post-reboot-node-labels keys do not match, resulting in unexpected behaviour")
	}

	return nil
}

// Remove old flags
func validateNotificationURL(notifyURL string, slackHookURL string) string {
	switch {
	case slackHookURL != "" && notifyURL != "":
		slog.Error(fmt.Sprintf("Cannot use both --notify-url (given: %v) and --slack-hook-url (given: %v) flags. Kured will only use --notify-url flag", slackHookURL, notifyURL))
		return validateNotificationURL(notifyURL, "")
	case notifyURL != "":
		return stripQuotes(notifyURL)
	case slackHookURL != "":
		slog.Info("Deprecated flag(s). Please use --notify-url flag instead.")
		parsedURL, err := url.Parse(stripQuotes(slackHookURL))
		if err != nil {
			slog.Info(fmt.Sprintf("slack-hook-url is not properly formatted... no notification will be sent: %v\n", err))
			return ""
		}
		if len(strings.Split(strings.ReplaceAll(parsedURL.Path, "/services/", ""), "/")) != 3 {
			slog.Error(fmt.Sprintf("slack-hook-url is not properly formatted... no notification will be sent: unexpected number of / in URL\n"))
			return ""
		}
		return fmt.Sprintf("slack://%s", strings.ReplaceAll(parsedURL.Path, "/services/", ""))
	}
	return ""
}

// LoadFromEnv attempts to load environment variables corresponding to flags.
// It looks for an environment variable with the uppercase version of the flag name (prefixed by EnvPrefix).
func LoadFromEnv() {
	flag.VisitAll(func(f *flag.Flag) {
		envVarName := fmt.Sprintf("%s_%s", EnvPrefix, strings.ToUpper(strings.ReplaceAll(f.Name, "-", "_")))

		if envValue, exists := os.LookupEnv(envVarName); exists {
			switch f.Value.Type() {
			case "int":
				if parsedVal, err := strconv.Atoi(envValue); err == nil {
					err := flag.Set(f.Name, strconv.Itoa(parsedVal))
					if err != nil {
						fmt.Printf("cannot set flag %s from env var named %s", f.Name, envVarName)
						os.Exit(1)
					} // Set int flag
				} else {
					fmt.Printf("Invalid value for env var named %s", envVarName)
					os.Exit(1)
				}
			case "string":
				err := flag.Set(f.Name, envValue)
				if err != nil {
					fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
					os.Exit(1)
				} // Set string flag
			case "bool":
				if parsedVal, err := strconv.ParseBool(envValue); err == nil {
					err := flag.Set(f.Name, strconv.FormatBool(parsedVal))
					if err != nil {
						fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
						os.Exit(1)
					} // Set boolean flag
				} else {
					fmt.Printf("Invalid value for %s: %s\n", envVarName, envValue)
					os.Exit(1)
				}
			case "duration":
				// Set duration from the environment variable (e.g., "1h30m")
				if _, err := time.ParseDuration(envValue); err == nil {
					err = flag.Set(f.Name, envValue)
					if err != nil {
						fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
						os.Exit(1)
					}
				} else {
					fmt.Printf("Invalid duration for %s: %s\n", envVarName, envValue)
					os.Exit(1)
				}
			case "regexp":
				// For regexp, set it from the environment variable
				err := flag.Set(f.Name, envValue)
				if err != nil {
					fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
					os.Exit(1)
				}
			case "stringSlice":
				// For stringSlice, split the environment variable by commas and set it
				err := flag.Set(f.Name, envValue)
				if err != nil {
					fmt.Printf("cannot set flag %s from env{%s}: %s\n", f.Name, envVarName, envValue)
					os.Exit(1)
				}
			default:
				fmt.Printf("Unsupported flag type for %s\n", f.Name)
			}
		}
	})

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

type slogWriter struct {
	stream  string
	message string
}

func (sw slogWriter) Write(p []byte) (n int, err error) {
	output := string(p)
	switch sw.stream {
	case "stdout":
		slog.Info(sw.message, "node", nodeID, "stdout", output)
	case "stderr":
		slog.Info(sw.message, "node", nodeID, "stderr", output)
	}
	return len(p), nil
}

func drain(client *kubernetes.Clientset, node *v1.Node) error {
	nodename := node.GetName()

	if preRebootNodeLabels != nil {
		err := updateNodeLabels(client, node, preRebootNodeLabels)
		if err != nil {
			return fmt.Errorf("stopping drain due to problem with node labels %v", err)
		}
	}

	if drainDelay > 0 {
		slog.Debug("Delaying drain", "delay", drainDelay, "node", nodename)
		time.Sleep(drainDelay)
	}

	slog.Info("Starting drain", "node", nodename)

	if notifyURL != "" {
		if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateDrain, nodename)); err != nil {
			slog.Info(fmt.Sprintf("Error notifying but continuing drain: %v", err), "node", nodename)
		}
	}

	kubectlStdOutLogger := &slogWriter{message: "draining: results", stream: "stdout"}
	kubectlStdErrLogger := &slogWriter{message: "draining: results", stream: "stderr"}

	drainer := &kubectldrain.Helper{
		Client:                          client,
		Ctx:                             context.Background(),
		GracePeriodSeconds:              drainGracePeriod,
		PodSelector:                     drainPodSelector,
		SkipWaitForDeleteTimeoutSeconds: skipWaitForDeleteTimeoutSeconds,
		Force:                           true,
		DeleteEmptyDirData:              true,
		IgnoreAllDaemonSets:             true,
		ErrOut:                          kubectlStdErrLogger,
		Out:                             kubectlStdOutLogger,
		Timeout:                         drainTimeout,
	}

	if err := kubectldrain.RunCordonOrUncordon(drainer, node, true); err != nil {
		return fmt.Errorf("error cordonning node %s, %v", nodename, err)
	}

	if err := kubectldrain.RunNodeDrain(drainer, nodename); err != nil {
		return fmt.Errorf("error draining node %s: %v", nodename, err)
	}
	return nil
}

func uncordon(client *kubernetes.Clientset, node *v1.Node) error {
	nodename := node.GetName()
	kubectlStdOutLogger := &slogWriter{message: "uncordon: results", stream: "stdout"}
	kubectlStdErrLogger := &slogWriter{message: "uncordon: results", stream: "stderr"}

	drainer := &kubectldrain.Helper{
		Client: client,
		ErrOut: kubectlStdErrLogger,
		Out:    kubectlStdOutLogger,
		Ctx:    context.Background(),
	}
	if err := kubectldrain.RunCordonOrUncordon(drainer, node, false); err != nil {
		return fmt.Errorf("error uncordonning node %s: %v", nodename, err)
	} else if postRebootNodeLabels != nil {
		err := updateNodeLabels(client, node, postRebootNodeLabels)
		return fmt.Errorf("error updating node (%s) labels, needs manual intervention %v", nodename, err)
	}
	return nil
}

func addNodeAnnotations(client *kubernetes.Clientset, nodeID string, annotations map[string]string) error {
	node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error retrieving node object via k8s API: %v", err)
	}
	for k, v := range annotations {
		node.Annotations[k] = v
		slog.Debug(fmt.Sprintf("adding node annotation: %s=%s", k, v), "node", node.GetName())
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("error marshalling node object into JSON: %v", err)
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), node.GetName(), types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		var annotationsErr string
		for k, v := range annotations {
			annotationsErr += fmt.Sprintf("%s=%s ", k, v)
		}
		return fmt.Errorf("error adding node annotations %s via k8s API: %v", annotationsErr, err)
	}
	return nil
}

func deleteNodeAnnotation(client *kubernetes.Clientset, nodeID, key string) error {
	// JSON Patch takes as path input a JSON Pointer, defined in RFC6901
	// So we replace all instances of "/" with "~1" as per:
	// https://tools.ietf.org/html/rfc6901#section-3
	patch := []byte(fmt.Sprintf("[{\"op\":\"remove\",\"path\":\"/metadata/annotations/%s\"}]", strings.ReplaceAll(key, "/", "~1")))
	_, err := client.CoreV1().Nodes().Patch(context.TODO(), nodeID, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error deleting node annotation %s via k8s API: %v", key, err)
	}
	return nil
}

func updateNodeLabels(client *kubernetes.Clientset, node *v1.Node, labels []string) error {
	labelsMap := make(map[string]string)
	for _, label := range labels {
		k := strings.Split(label, "=")[0]
		v := strings.Split(label, "=")[1]
		labelsMap[k] = v
		slog.Debug(fmt.Sprintf("Updating node %s label: %s=%s", node.GetName(), k, v), "node", node.GetName())
	}

	bytes, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"labels": labelsMap,
		},
	})
	if err != nil {
		return fmt.Errorf("error marshalling node object into JSON: %v", err)
	}

	_, err = client.CoreV1().Nodes().Patch(context.TODO(), node.GetName(), types.StrategicMergePatchType, bytes, metav1.PatchOptions{})
	if err != nil {
		var labelsErr string
		for _, label := range labels {
			k := strings.Split(label, "=")[0]
			v := strings.Split(label, "=")[1]
			labelsErr += fmt.Sprintf("%s=%s ", k, v)
		}
		return fmt.Errorf("error updating node labels %s via k8s API: %v", labelsErr, err)
	}
	return nil
}

func rebootAsRequired(nodeID string, rebooter reboot.Rebooter, checker checkers.Checker, blockCheckers []blockers.RebootBlocker, window *timewindow.TimeWindow, lock daemonsetlock.Lock, client *kubernetes.Clientset, period time.Duration) {

	preferNoScheduleTaint := taints.New(client, nodeID, preferNoScheduleTaintName, v1.TaintEffectPreferNoSchedule)

	// No reason to delay the first ticks.
	// On top of it, we used to leak a goroutine, which was never garbage collected.
	// Starting on go1.23, with Tick, we would have that goroutine garbage collected.
	c := time.Tick(period)
	for range c {
		rebootRequired := checker.RebootRequired()
		if !rebootRequired {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(0)
			// Now cleaning up post reboot

			// Quickly allow rescheduling.
			// The node could be still cordonned anyway
			preferNoScheduleTaint.Disable()

			// Test API server first. If cannot get node, we should not do anything.
			node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
			if err != nil {
				// Only debug level even though the API is dead: Kured should not worry about transient
				// issues, the k8s cluster admin should be aware already
				slog.Debug(fmt.Sprintf("error retrieving node object via k8s API: %v.\nPlease check API", err), "node", nodeID, "error", err)
				continue
			}

			// Get lock data to know if need to uncordon the node
			// to get the node back to its previous state
			// TODO: Need to move to another method to check the current data of the lock relevant for this node
			holding, lockData, err := lock.Holding()
			if err != nil {
				// Internal wiring, admin does not need to be aware unless debugging session
				slog.Debug(fmt.Sprintf("Error checking lock - Not applying any action: %v.\nPlease check API", err), "node", nodeID, "error", err)
				continue
			}

			// we check if holding ONLY to know if lockData is valid.
			// When moving to fetch lockData as a separate method, remove
			// this whole condition.
			// However, it means that Release()
			// need to behave idempotently regardless or not the lock is
			// held, but that's an ideal state.
			// what we should simply do is reconcile the lock data
			// with the node spec. But behind the scenes its a bug
			// if it's not holding due to an error
			if holding && !lockData.Metadata.Unschedulable {
				// Split into two lines to remember I need to remove the first
				// condition ;)
				if node.Spec.Unschedulable != lockData.Metadata.Unschedulable && lockData.Metadata.Unschedulable == false {
					// Important lifecycle event - admin should be aware
					slog.Info("uncordoning node", "node", nodeID)
					err = uncordon(client, node)
					if err != nil {
						// Transient API issue - no problem, will retry. No need to worry the admin for this
						slog.Debug(fmt.Sprintf("Unable to uncordon %s: %v, will continue to hold lock and retry uncordon", node.GetName(), err), "node", nodeID, "error", err)
						continue
					}
					// TODO, modify the actions to directly log and notify, instead of individual methods giving
					// an incomplete view of the lifecycle
					if notifyURL != "" {
						if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateUncordon, nodeID)); err != nil {
							// admin might want to act upon this -- raising to info level
							slog.Info(fmt.Sprintf("Error notifying: %v", err), "node", nodeID, "error", err)
						}
					}
				}

			}

			// Releasing lock earlier is nice for other nodes
			err = lock.Release()
			if err != nil {
				// Lock release is an internal thing, do not worry the admin too much
				slog.Debug("Error releasing lock, will retry", "node", nodeID, "error", err)
				continue
			}
			// Regardless or not we are holding the lock
			// The node should not say it's still in progress if the reboot is done
			if annotateNodes {
				if _, ok := node.Annotations[KuredRebootInProgressAnnotation]; ok {
					// Who reads this? I hope nobody bothers outside real debug cases
					slog.Debug(fmt.Sprintf("Deleting node %s annotation %s", nodeID, KuredRebootInProgressAnnotation), "node", nodeID)
					err := deleteNodeAnnotation(client, nodeID, KuredRebootInProgressAnnotation)
					if err != nil {
						continue
					}
				}
			}

		} else {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(1)

			// Act on reboot required.
			if !window.Contains(time.Now()) {
				// Probably spamming outside the maintenance window. This should not be logged as info
				slog.Debug("reboot required but outside maintenance window", "node", nodeID)
				continue
			}
			// moved up, because we should not put an annotation "Going to be rebooting", if
			// we know well that this won't reboot. TBD as some ppl might have another opinion.

			if blocked, currentlyBlocking := blockers.RebootBlocked(blockCheckers...); blocked {
				for _, blocker := range currentlyBlocking {
					rebootBlockedCounter.WithLabelValues(nodeID, blocker.MetricLabel()).Inc()
					// Important lifecycle event -- tried to reboot, but was blocked!
					slog.Info(fmt.Sprintf("reboot blocked by %v", blocker.MetricLabel()), "node", nodeID)
				}
				continue
			}

			// Test API server first. If cannot get node, we should not do anything.
			node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
			if err != nil {
				// Not important to worry the admin
				slog.Debug("error retrieving node object via k8s API, retrying at next tick", "node", nodeID, "error", err)
				continue
			}
			// nodeMeta contains the node Metadata that should be included in the lock
			nodeMeta := daemonsetlock.NodeMeta{Unschedulable: node.Spec.Unschedulable}

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

			// Prefer to not schedule pods onto this node to avoid draing the same pod multiple times.
			preferNoScheduleTaint.Enable()

			// This could be merged into a single idempotent "Acquire" lock
			holding, _, err := lock.Holding()
			if err != nil {
				// Not important to worry the admin
				slog.Debug("error testing lock", "node", nodeID, "error", err)
				continue
			}

			if !holding {
				acquired, holder, err := lock.Acquire(nodeMeta)
				if err != nil {
					// Not important to worry the admin
					slog.Debug("error acquiring lock, will retry at next tick", "node", nodeID, "error", err)
					continue
				}
				if !acquired {
					// Not very important - lock prevents action
					slog.Debug(fmt.Sprintf("Lock already held by %v, will retry at next tick", holder), "node", nodeID)
					continue
				}
			}

			err = drain(client, node)
			if err != nil {
				if !forceReboot {
					slog.Debug(fmt.Sprintf("Unable to cordon or drain %s: %v, will force-reboot by releasing lock and uncordon until next success", node.GetName(), err), "node", nodeID, "error", err)
					err = lock.Release()
					if err != nil {
						slog.Debug(fmt.Sprintf("error in best-effort releasing lock: %v", err), "node", nodeID, "error", err)
					}
					// this is important -- if it's not shown, it means that (on normal / non-force reboot case) the drain was
					// in error, the lock was NOT released.
					// If shown, it is helping understand the uncordonning. If the admin seems the node as cordonned
					// with this, it needs to take action (for example if the node was previously cordonned!)
					slog.Info("Performing a best-effort uncordon after failed cordon and drain", "node", nodeID)
					err := uncordon(client, node)
					if err != nil {
						slog.Info("Failed to do best-effort uncordon", "node", nodeID, "error", err)
					}
					continue
				}
			}
			if notifyURL != "" {
				if err := shoutrrr.Send(notifyURL, fmt.Sprintf(messageTemplateReboot, nodeID)); err != nil {
					// most likely important to see: if not notified, at least it would be logged!
					slog.Info("Error sending notification for reboot", "node", nodeID, "error", err)
				}
			}
			// important lifecycle event
			slog.Info(fmt.Sprintf("Triggering reboot for node %v", nodeID), "node", nodeID)

			err := rebooter.Reboot()
			if err != nil {
				slog.Info("Error rebooting node", "node", nodeID, "error", err)
				continue
			}
			for {
				slog.Info("Waiting for reboot", "node", nodeID)
				time.Sleep(time.Minute)
			}
		}
	}
}
