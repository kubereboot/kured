package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/kubereboot/kured/internal"
	"github.com/kubereboot/kured/pkg/blockers"
	"github.com/kubereboot/kured/pkg/checkers"
	"github.com/kubereboot/kured/pkg/daemonsetlock"
	"github.com/kubereboot/kured/pkg/delaytick"
	"github.com/kubereboot/kured/pkg/reboot"
	"github.com/kubereboot/kured/pkg/taints"
	"github.com/kubereboot/kured/pkg/timewindow"
	papi "github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
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
	prometheus.MustRegister(rebootRequiredGauge)
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
	flag.DurationVar(&period, "period", time.Minute*60,
		"sentinel check period")
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

	log.Infof("Kubernetes Reboot Daemon: %s", version)

	if logFormat == "json" {
		log.SetFormatter(&log.JSONFormatter{})
	}

	if nodeID == "" {
		log.Fatal("KURED_NODE_ID environment variable required")
	}
	log.Infof("Node ID: %s", nodeID)

	notifyURL = validateNotificationURL(notifyURL, slackHookURL)

	err := validateNodeLabels(preRebootNodeLabels, postRebootNodeLabels)
	if err != nil {
		log.Warnf(err.Error())
	}

	log.Infof("PreferNoSchedule taint: %s", preferNoScheduleTaintName)

	// This should be printed from blocker list instead of only blocking pod selectors
	log.Infof("Blocking Pod Selectors: %v", podSelectors)

	log.Infof("Reboot period %v", period)
	log.Infof("Concurrency: %v", concurrency)

	if annotateNodes {
		log.Infof("Will annotate nodes during kured reboot operations")
	}

	// Now call the rest of the main loop.
	window, err := timewindow.New(rebootDays, rebootStart, rebootEnd, timezone)
	if err != nil {
		log.Fatalf("Failed to build time window: %v", err)
	}
	log.Infof("Reboot schedule: %v", window)

	log.Infof("Reboot method: %s", rebootMethod)
	rebooter, err := internal.NewRebooter(rebootMethod, rebootCommand, rebootSignal)
	if err != nil {
		log.Fatalf("Failed to build rebooter: %v", err)
	}

	rebootChecker, err := internal.NewRebootChecker(rebootSentinelCommand, rebootSentinelFile)
	if err != nil {
		log.Fatalf("Failed to build reboot checker: %v", err)
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	var blockCheckers []blockers.RebootBlocker
	if prometheusURL != "" {
		blockCheckers = append(blockCheckers, blockers.NewPrometheusBlockingChecker(papi.Config{Address: prometheusURL}, alertFilter.Regexp, alertFiringOnly, alertFilterMatchOnly))
	}
	if podSelectors != nil {
		blockCheckers = append(blockCheckers, blockers.NewKubernetesBlockingChecker(client, nodeID, podSelectors))
	}
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
	lock := daemonsetlock.New(client, nodeID, dsNamespace, dsName, lockAnnotation, lockTTL, concurrency, lockReleaseDelay)

	go rebootAsRequired(nodeID, rebooter, rebootChecker, blockCheckers, window, lock, client)
	go maintainRebootRequiredMetric(nodeID, rebootChecker)

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(fmt.Sprintf("%s:%d", metricsHost, metricsPort), nil))
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

func validateNotificationURL(notifyURL string, slackHookURL string) string {
	switch {
	case slackHookURL != "" && notifyURL != "":
		log.Warnf("Cannot use both --notify-url (given: %v) and --slack-hook-url (given: %v) flags. Kured will only use --notify-url flag", slackHookURL, notifyURL)
		return validateNotificationURL(notifyURL, "")
	case notifyURL != "":
		return stripQuotes(notifyURL)
	case slackHookURL != "":
		log.Warnf("Deprecated flag(s). Please use --notify-url flag instead.")
		parsedURL, err := url.Parse(stripQuotes(slackHookURL))
		if err != nil {
			log.Errorf("slack-hook-url is not properly formatted... no notification will be sent: %v\n", err)
			return ""
		}
		if len(strings.Split(strings.Replace(parsedURL.Path, "/services/", "", -1), "/")) != 3 {
			log.Errorf("slack-hook-url is not properly formatted... no notification will be sent: unexpected number of / in URL\n")
			return ""
		}
		return fmt.Sprintf("slack://%s", strings.Replace(parsedURL.Path, "/services/", "", -1))
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
					flag.Set(f.Name, envValue)
				} else {
					fmt.Printf("Invalid duration for %s: %s\n", envVarName, envValue)
					os.Exit(1)
				}
			case "regexp":
				// For regexp, set it from the environment variable
				flag.Set(f.Name, envValue)
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

func maintainRebootRequiredMetric(nodeID string, checker checkers.Checker) {
	for {
		if checker.RebootRequired() {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(1)
		} else {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(0)
		}
		time.Sleep(time.Minute)
	}
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

func rebootAsRequired(nodeID string, rebooter reboot.Rebooter, checker checkers.Checker, blockCheckers []blockers.RebootBlocker, window *timewindow.TimeWindow, lock daemonsetlock.Lock, client *kubernetes.Clientset) {

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, 1*time.Minute)
	for range tick {
		holding, lockData, err := lock.Holding()
		if err != nil {
			log.Errorf("Error testing lock: %v", err)
		}
		if holding {
			node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
			if err != nil {
				log.Errorf("Error retrieving node object via k8s API: %v", err)
				continue
			}

			if !lockData.Metadata.Unschedulable {
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
			if annotateNodes && !checker.RebootRequired() {
				if _, ok := node.Annotations[KuredRebootInProgressAnnotation]; ok {
					err := deleteNodeAnnotation(client, nodeID, KuredRebootInProgressAnnotation)
					if err != nil {
						continue
					}
				}
			}

			err = lock.Release()
			if err != nil {
				log.Errorf("Error releasing lock, will retry: %v", err)
				continue
			}
			break
		} else {
			break
		}
	}

	preferNoScheduleTaint := taints.New(client, nodeID, preferNoScheduleTaintName, v1.TaintEffectPreferNoSchedule)

	// Remove taint immediately during startup to quickly allow scheduling again.
	if !checker.RebootRequired() {
		preferNoScheduleTaint.Disable()
	}

	source = rand.NewSource(time.Now().UnixNano())
	tick = delaytick.New(source, period)
	for range tick {
		if !window.Contains(time.Now()) {
			// Remove taint outside the reboot time window to allow for normal operation.
			preferNoScheduleTaint.Disable()
			continue
		}

		if !checker.RebootRequired() {
			log.Infof("Reboot not required")
			preferNoScheduleTaint.Disable()
			continue
		}

		node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
		if err != nil {
			log.Fatalf("Error retrieving node object via k8s API: %v", err)
		}

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

		var rebootRequiredBlockCondition string
		if blockers.RebootBlocked(blockCheckers...) {
			rebootRequiredBlockCondition = ", but blocked at this time"
			continue
		}
		log.Infof("Reboot required%s", rebootRequiredBlockCondition)

		holding, _, err := lock.Holding()
		if err != nil {
			log.Errorf("Error testing lock: %v", err)
		}

		if !holding {
			acquired, holder, err := lock.Acquire(nodeMeta)
			if err != nil {
				log.Errorf("Error acquiring lock: %v", err)
			}
			if !acquired {
				log.Warnf("Lock already held: %v", holder)
				// Prefer to not schedule pods onto this node to avoid draing the same pod multiple times.
				preferNoScheduleTaint.Enable()
				continue
			}
		}

		err = drain(client, node)
		if err != nil {
			if !forceReboot {
				log.Errorf("Unable to cordon or drain %s: %v, will release lock and retry cordon and drain before rebooting when lock is next acquired", node.GetName(), err)
				err = lock.Release()
				if err != nil {
					log.Errorf("Error releasing lock: %v", err)
				}
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
		log.Infof("Triggering reboot for node %v", nodeID)

		err = rebooter.Reboot()
		if err != nil {
			log.Fatalf("Unable to reboot node: %v", err)
		}
		for {
			log.Infof("Waiting for reboot")
			time.Sleep(time.Minute)
		}
	}
}
