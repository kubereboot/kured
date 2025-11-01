// The main controller for kured
// This package is a reference implementation on how to reboot your nodes based on the different
// tools present in this project's modules
package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/kubereboot/kured/internal/cli"
	"github.com/kubereboot/kured/internal/daemonsetlock"
	"github.com/kubereboot/kured/internal/k8soperations"
	"github.com/kubereboot/kured/internal/notifications"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	version = "unreleased"

	// Command line flags (sorted alphabetically)
	alertFilter                     cli.RegexpValue
	alertFilterMatchOnly            bool
	alertFiringOnly                 bool
	annotateNodeProgress            bool
	concurrency                     int
	drainDelay                      time.Duration
	drainGracePeriod                int
	drainPodSelector                string
	drainTimeout                    time.Duration
	dsName                          string
	dsNamespace                     string
	lockAnnotation                  string
	lockReleaseDelay                time.Duration
	lockTTL                         time.Duration
	logFormat                       string
	messageTemplateDrain            string
	messageTemplateReboot           string
	messageTemplateUncordon         string
	metricsHost                     string
	metricsPort                     int
	nodeID                          string
	notifyURLs                      []string
	period                          time.Duration
	podSelectors                    []string
	postRebootNodeLabels            []string
	preRebootNodeLabels             []string
	preferNoScheduleTaintName       string
	prometheusURL                   string
	rebootCommand                   string
	rebootDays                      []string
	rebootDelay                     time.Duration
	rebootEnd                       string
	rebootMethod                    string
	rebootSentinelCommand           string
	rebootSentinelFile              string
	rebootSignal                    int
	rebootStart                     string
	skipWaitForDeleteTimeoutSeconds int
	timezone                        string
	forceReboot                     bool

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
	KuredNodeLockAnnotation string = "kured.dev/kured-node-lock"
	// KuredRebootInProgressAnnotation is the canonical string value for the kured reboot-in-progress annotation
	KuredRebootInProgressAnnotation string = "kured.dev/kured-reboot-in-progress"
	// KuredMostRecentRebootNeededAnnotation is the canonical string value for the kured most-recent-reboot-needed annotation
	KuredMostRecentRebootNeededAnnotation string = "kured.dev/kured-most-recent-reboot-needed"
	// TODO: Replace this with runtime evaluation
	sigRTMinPlus5 = 34 + 5
)

func init() {
	prometheus.MustRegister(rebootRequiredGauge, rebootBlockedCounter)
}

func main() {

	// flags are sorted alphabetically by type
	flag.BoolVar(&alertFilterMatchOnly, "alert-filter-match-only", false, "Only block if the alert-filter-regexp matches active alerts")
	flag.BoolVar(&alertFiringOnly, "alert-firing-only", false, "only consider firing alerts when checking for active alerts")
	flag.BoolVar(&annotateNodeProgress, "annotate-nodes", false, "if set, the annotations 'kured.dev/kured-reboot-in-progress' and 'kured.dev/kured-most-recent-reboot-needed' will be given to nodes undergoing kured reboots")
	flag.BoolVar(&forceReboot, "force-reboot", false, "force a reboot even if the drain fails or times out")
	flag.DurationVar(&drainDelay, "drain-delay", 0, "delay drain for this duration (default: 0, disabled)")
	flag.DurationVar(&drainTimeout, "drain-timeout", 0, "timeout after which the drain is aborted (default: 0, infinite time)")
	flag.DurationVar(&lockReleaseDelay, "lock-release-delay", 0, "delay lock release for this duration (default: 0, disabled)")
	flag.DurationVar(&lockTTL, "lock-ttl", 0, "expire lock annotation after this duration (default: 0, disabled)")
	flag.DurationVar(&period, "period", time.Minute, "period at which the main operations are done")
	flag.DurationVar(&rebootDelay, "reboot-delay", 0, "delay reboot for this duration (default: 0, disabled)")
	flag.IntVar(&concurrency, "concurrency", 1, "amount of nodes to concurrently reboot. Defaults to 1")
	flag.IntVar(&drainGracePeriod, "drain-grace-period", -1, "time in seconds given to each pod to terminate gracefully, if negative, the default value specified in the pod will be used")
	flag.IntVar(&metricsPort, "metrics-port", 8080, "port number where metrics will listen")
	flag.IntVar(&rebootSignal, "reboot-signal", sigRTMinPlus5, "signal to use for reboot, SIGRTMIN+5 by default.")
	flag.IntVar(&skipWaitForDeleteTimeoutSeconds, "skip-wait-for-delete-timeout", 0, "when seconds is greater than zero, skip waiting for the pods whose deletion timestamp is older than N seconds while draining a node")
	flag.StringArrayVar(&notifyURLs, "notify-url", nil, "notify URL for reboot notifications (can be repeated for multiple notifications)")
	flag.StringArrayVar(&podSelectors, "blocking-pod-selector", nil, "label selector identifying pods whose presence should prevent reboots")
	flag.StringSliceVar(&postRebootNodeLabels, "post-reboot-node-labels", nil, "labels to add to nodes after uncordoning")
	flag.StringSliceVar(&preRebootNodeLabels, "pre-reboot-node-labels", nil, "labels to add to nodes before cordoning")
	flag.StringSliceVar(&rebootDays, "reboot-days", timewindow.EveryDay, "schedule reboot on these days")
	flag.StringVar(&drainPodSelector, "drain-pod-selector", "", "only drain pods with labels matching the selector (default: '', all pods)")
	flag.StringVar(&dsName, "ds-name", "kured", "name of daemonset on which to place lock")
	flag.StringVar(&dsNamespace, "ds-namespace", "kube-system", "namespace containing daemonset on which to place lock")
	flag.StringVar(&lockAnnotation, "lock-annotation", KuredNodeLockAnnotation, "annotation in which to record locking node")
	flag.StringVar(&logFormat, "log-format", "text", "use text or json log format")
	flag.StringVar(&messageTemplateDrain, "message-template-drain", "Draining node %s", "message template used to notify about a node being drained")
	flag.StringVar(&messageTemplateReboot, "message-template-reboot", "Rebooting node %s", "message template used to notify about a node being rebooted")
	flag.StringVar(&messageTemplateUncordon, "message-template-uncordon", "Node %s rebooted & uncordoned successfully!", "message template used to notify about a node being successfully uncordoned")
	flag.StringVar(&metricsHost, "metrics-host", "", "host where metrics will listen")
	flag.StringVar(&nodeID, "node-id", "", "node name kured runs on, should be passed down from spec.nodeName via KURED_NODE_ID environment variable")
	flag.StringVar(&preferNoScheduleTaintName, "prefer-no-schedule-taint", "", "Taint name applied during pending node reboot (to prevent receiving additional pods from other rebooting nodes). Disabled by default. Set e.g. to \"kured.dev/kured-node-reboot\" to enable tainting.")
	flag.StringVar(&prometheusURL, "prometheus-url", "", "Prometheus instance to probe for active alerts")
	flag.StringVar(&rebootCommand, "reboot-command", "/bin/systemctl reboot", "command to run when a reboot is required")
	flag.StringVar(&rebootEnd, "end-time", "23:59:59", "schedule reboot only before this time of day")
	flag.StringVar(&rebootMethod, "reboot-method", "command", "method to use for reboots. Available: command")
	flag.StringVar(&rebootSentinelCommand, "reboot-sentinel-command", "", "command for which a zero return code will trigger a reboot command")
	flag.StringVar(&rebootSentinelFile, "reboot-sentinel", "/var/run/reboot-required", "path to file whose existence triggers the reboot command")
	flag.StringVar(&rebootStart, "start-time", "0:00", "schedule reboot only after this time of day")
	flag.StringVar(&timezone, "time-zone", "UTC", "use this timezone for schedule inputs")
	flag.Var(&alertFilter, "alert-filter-regexp", "alert names to ignore when checking for active alerts")

	flag.Parse()

	// Load flags from environment variables
	cli.LoadFromEnv()

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

	notifier := notifications.NewNotifier(notifyURLs...)

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
		"taint", fmt.Sprintf("preferNoSchedule taint: (%s)", preferNoScheduleTaintName),
		"annotation", fmt.Sprintf("Lock Annotation: %s", lockAnnotation),
	)

	if annotateNodeProgress {
		slog.Info("Will annotate nodes progress during kured reboot operations", "node", nodeID)
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

	go rebootAsRequired(nodeID, rebooter, rebootChecker, blockCheckers, window, lock, client, period, notifier)

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

func rebootAsRequired(nodeID string, rebooter reboot.Rebooter, checker checkers.Checker, blockCheckers []blockers.RebootBlocker, window *timewindow.TimeWindow, lock daemonsetlock.Lock, client *kubernetes.Clientset, period time.Duration, notifier notifications.Notifier) {

	preferNoScheduleTaint := k8soperations.NewTaint(client, nodeID, preferNoScheduleTaintName, v1.TaintEffectPreferNoSchedule)

	// No reason to delay the first ticks.
	// On top of it, we used to leak a goroutine, which was never garbage collected.
	// Starting on go1.23, with Tick, we would have that goroutine garbage collected.
	c := time.Tick(period)
	for range c {
		rebootRequired := checker.RebootRequired()
		if !rebootRequired {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(0)
			// Now cleaning up after a reboot

			// Quickly allow rescheduling.
			// The node could be still cordonned anyway
			preferNoScheduleTaint.Disable()

			// Test the API server first. If we cannot get node, we should not do anything.
			node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
			if err != nil {
				// Only debug level even though the API is dead: Kured should not worry about transient
				// issues, the k8s cluster admin should be aware already
				slog.Debug(fmt.Sprintf("error retrieving node object via k8s API: %v.\nPlease check API", err), "node", nodeID, "error", err)
				continue
			}

			err = k8soperations.Uncordon(client, node, notifier, postRebootNodeLabels, messageTemplateUncordon)
			if err != nil {
				// Might be a transient API issue or a real problem. Inform the admin
				slog.Info("unable to uncordon needs investigation", "node", nodeID, "error", err)
				continue
			}

			// Releasing lock earlier is nice for other nodes
			err = lock.Release()
			if err != nil {
				// Lock release is an internal thing, do not worry the admin too much
				slog.Debug("Error releasing lock, will retry", "node", nodeID, "error", err)
				continue
			}
			// Do this regardless or not we are holding the lock
			// The node should not say it's still in progress if the reboot is done
			if annotateNodeProgress {
				if _, ok := node.Annotations[KuredRebootInProgressAnnotation]; ok {
					// Who reads this? I hope nobody bothers outside real debug cases
					slog.Debug(fmt.Sprintf("Deleting node %s annotation %s", nodeID, KuredRebootInProgressAnnotation), "node", nodeID)
					err := k8soperations.DeleteNodeAnnotation(client, nodeID, KuredRebootInProgressAnnotation)
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
			// moved up because we should not put an annotation "Going to be rebooting", if
			// we know well that this won't reboot. TBD as some ppl might have another opinion.

			if blocked, currentlyBlocking := blockers.RebootBlocked(blockCheckers...); blocked {
				for _, blocker := range currentlyBlocking {
					rebootBlockedCounter.WithLabelValues(nodeID, blocker.MetricLabel()).Inc()
					// Important lifecycle event -- tried to reboot, but was blocked!
					slog.Info(fmt.Sprintf("reboot blocked by %v", blocker.MetricLabel()), "node", nodeID)
				}
				continue
			}

			// Test the API server first. If we cannot get node, we should not do anything.
			node, err := client.CoreV1().Nodes().Get(context.TODO(), nodeID, metav1.GetOptions{})
			if err != nil {
				// Not important enough to worry the admin
				slog.Debug("error retrieving node object via k8s API, retrying at next tick", "node", nodeID, "error", err)
				continue
			}

			var timeNowString string
			if annotateNodeProgress {
				if _, ok := node.Annotations[KuredRebootInProgressAnnotation]; !ok {
					timeNowString = time.Now().Format(time.RFC3339)
					// Annotate this node to indicate that "I am going to be rebooted!"
					// so that other node maintenance tools running on the cluster are aware that this node is in the process of a "state transition"
					annotations := map[string]string{KuredRebootInProgressAnnotation: timeNowString}
					// & annotate this node with a timestamp so that other node maintenance tools know how long it's been since this node has been marked for reboot
					annotations[KuredMostRecentRebootNeededAnnotation] = timeNowString
					err := k8soperations.AddNodeAnnotations(client, nodeID, annotations)
					if err != nil {
						continue
					}
				}
			}

			// Prefer to not schedule pods onto this node to avoid draining the same pod multiple times.
			preferNoScheduleTaint.Enable()

			holding, err := lock.Acquire()
			if err != nil || !holding {
				slog.Debug("error acquiring lock, will retry at next tick", "node", nodeID, "error", err)
				continue
			}
			//// This could be merged into a single idempotent "Acquire" lock
			//holding, _, err := lock.Holding()
			//if err != nil {
			//	// Not important to worry the admin
			//	slog.Debug("error testing lock", "node", nodeID, "error", err)
			//	continue
			//}
			//
			//if !holding {
			//	acquired, holder, err := lock.Acquire(nodeMeta)
			//	if err != nil {
			//		// Not important to worry the admin
			//		slog.Debug("error acquiring lock, will retry at next tick", "node", nodeID, "error", err)
			//		continue
			//	}
			//	if !acquired {
			//		// Not very important - lock prevents action
			//		slog.Debug(fmt.Sprintf("Lock already held by %v, will retry at next tick", holder), "node", nodeID)
			//		continue
			//	}
			//}

			err = k8soperations.Drain(client, node, preRebootNodeLabels, drainTimeout, drainGracePeriod, skipWaitForDeleteTimeoutSeconds, drainPodSelector, drainDelay, messageTemplateDrain, notifier)

			if err != nil {
				if !forceReboot {
					slog.Debug(fmt.Sprintf("Unable to cordon or drain %s: %v, will force-reboot by releasing lock and uncordon until next success", node.GetName(), err), "node", nodeID, "error", err)
					err = lock.Release()
					if err != nil {
						slog.Debug(fmt.Sprintf("error in best-effort releasing lock: %v", err), "node", nodeID, "error", err)
					}
					// this is important -- if the next info not shown, it means that (in a normal or non-force reboot case)
					// the drain was in error and the lock was NOT released.
					// If shown, it is helping understand the "uncordoning".
					// If the admin seems the node as cordoned even after trying a best-effort uncordon,
					// the admin needs to take action (especially if the node was previously cordoned before the maintenance!)
					slog.Info("Performing a best-effort uncordon after failed cordon and drain", "node", nodeID)
					err := k8soperations.Uncordon(client, node, notifier, postRebootNodeLabels, messageTemplateUncordon)
					if err != nil {
						slog.Info("Failed to do best-effort uncordon", "node", nodeID, "error", err)
					}
					continue
				}
			}
			if err := notifier.Send(fmt.Sprintf(messageTemplateReboot, nodeID), "Node Reboot"); err != nil {
				// If the notifications are not sent, the lifecycle event of the reboot will be still recorded through
				// logging. Logging should not exceed the "debug" level.
				slog.Debug("Error sending notification for reboot", "node", nodeID, "error", err)
			}

			// important lifecycle event
			slog.Info(fmt.Sprintf("Triggering reboot for node %v", nodeID), "node", nodeID)

			if err := rebooter.Reboot(); err != nil {
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
