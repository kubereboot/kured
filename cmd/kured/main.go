package main

import (
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"

	"github.com/weaveworks/kured/pkg/alerts"
	"github.com/weaveworks/kured/pkg/daemonsetlock"
	"github.com/weaveworks/kured/pkg/delaytick"
)

var (
	version        = "unreleased"
	period         int
	dsNamespace    string
	dsName         string
	lockAnnotation string
	prometheusURL  string
	alertFilter    *regexp.Regexp
	rebootSentinel string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "kured",
		Short: "Kubernetes Reboot Daemon",
		Run:   root}

	rootCmd.PersistentFlags().IntVar(&period, "period", 60,
		"reboot check period in minutes")
	rootCmd.PersistentFlags().StringVar(&dsNamespace, "ds-name", "kube-system",
		"namespace containing daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&dsName, "ds-namespace", "kured",
		"name of daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&lockAnnotation, "lock-annotation", "weave.works/kured-node-lock",
		"annotation in which to record locking node")
	rootCmd.PersistentFlags().StringVar(&prometheusURL, "prometheus-url", "",
		"Prometheus instance to probe for active alerts")
	rootCmd.PersistentFlags().Var(&regexpValue{&alertFilter}, "alert-filter-regexp",
		"alert names to ignore when checking for active alerts")
	rootCmd.PersistentFlags().StringVar(&rebootSentinel, "reboot-sentinel", "/var/run/reboot-required",
		"path to file whose existence signals need to reboot")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func rebootRequired() bool {
	_, err := os.Stat(rebootSentinel)
	switch {
	case err == nil:
		log.Infof("Reboot required")
		return true
	case os.IsNotExist(err):
		log.Infof("Reboot not required")
		return false
	default:
		log.Fatalf("Unable to determine if reboot required: %v", err)
		return false // unreachable; prevents compilation error
	}
}

func rebootBlocked() bool {
	if prometheusURL != "" {
		count, err := alerts.PrometheusCountActive(prometheusURL, alertFilter)
		if err != nil {
			log.Warnf("Reboot blocked: prometheus query error: %v", err)
			return true
		}
		if count > 0 {
			log.Warnf("Reboot blocked: %d active alerts", count)
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

func acquire(lock *daemonsetlock.DaemonSetLock, metadata interface{}) bool {
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

func release(lock *daemonsetlock.DaemonSetLock) {
	log.Infof("Releasing lock")
	if err := lock.Release(); err != nil {
		log.Fatalf("Error releasing lock: %v", err)
	}
}

func drain(nodeID string) {
	log.Infof("Draining node %s", nodeID)
	drainCmd := exec.Command("/usr/bin/kubectl", "drain",
		"--ignore-daemonsets", "--delete-local-data", "--force", nodeID)
	if err := drainCmd.Run(); err != nil {
		log.Fatalf("Error invoking drain command: %v", err)
	}
}

func uncordon(nodeID string) {
	log.Infof("Uncordoning node %s", nodeID)
	uncordonCmd := exec.Command("/usr/bin/kubectl", "uncordon", nodeID)
	if err := uncordonCmd.Run(); err != nil {
		log.Fatalf("Error invoking uncordon command: %v", err)
	}
}

func waitForDrain(client *kubernetes.Clientset, nodeID string) {
	for {
		var unterminated int

		namespaces, err := client.CoreV1().Namespaces().List(v1.ListOptions{})
		if err != nil {
			log.Fatalf("Error waiting for drain: %v", err)
		}

		for _, namespace := range namespaces.Items {
			drainCandidates := v1.ListOptions{LabelSelector: "ignore_on_drain!=true"}
			pods, err := client.CoreV1().Pods(namespace.ObjectMeta.Name).List(drainCandidates)
			if err != nil {
				log.Fatalf("Error waiting for drain: %v", err)
			}

			for _, pod := range pods.Items {
				if pod.Spec.NodeName == nodeID &&
					pod.Status.Phase != "Succeeded" &&
					pod.Status.Phase != "Failed" {
					unterminated++
				}
			}
		}

		if unterminated == 0 {
			return
		}

		log.Infof("Waiting for %d pods to terminate", unterminated)
		time.Sleep(time.Minute)
	}
}

func reboot() {
	log.Infof("Commanding reboot")
	// Relies on /var/run/dbus/system_bus_socket bind mount to talk to systemd
	rebootCmd := exec.Command("/bin/systemctl", "reboot")
	if err := rebootCmd.Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}

func waitForReboot() {
	for {
		log.Infof("Waiting for reboot")
		time.Sleep(time.Minute)
	}
}

// nodeMeta is used to remember information across reboots
type nodeMeta struct {
	Unschedulable bool `json:"unschedulable"`
}

func root(cmd *cobra.Command, args []string) {
	log.Infof("Kubernetes Reboot Daemon: %s", version)

	nodeID := os.Getenv("KURED_NODE_ID")
	if nodeID == "" {
		log.Fatal("KURED_NODE_ID environment variable required")
	}

	log.Infof("Node ID: %s", nodeID)
	log.Infof("Lock Annotation: %s/%s:%s", dsNamespace, dsName, lockAnnotation)
	log.Infof("Reboot Sentinel: %s every %d minutes", rebootSentinel, period)

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
			uncordon(nodeID)
		}
		release(lock)
	} else {
		node, err := client.CoreV1().Nodes().Get(nodeID)
		if err != nil {
			log.Fatal(err)
		}
		nodeMeta.Unschedulable = node.Spec.Unschedulable
	}

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, time.Minute*time.Duration(period))
	for _ = range tick {
		if rebootRequired() && !rebootBlocked() && acquire(lock, &nodeMeta) {
			if !nodeMeta.Unschedulable {
				drain(nodeID)
				waitForDrain(client, nodeID)
			}
			reboot()
			break
		}
	}

	waitForReboot()
}
