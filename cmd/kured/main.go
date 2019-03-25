package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/weaveworks/kured/pkg/alerts"
	"github.com/weaveworks/kured/pkg/daemonsetlock"
	"github.com/weaveworks/kured/pkg/delaytick"
	"github.com/weaveworks/kured/pkg/notifications/slack"
)

// nodeMeta is used to remember information across reboots
type nodeMeta struct {
	Unschedulable bool `json:"unschedulable"`
}

var (
	version = "unreleased"

	// Command line flags
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

	// Kubernetes specific
	nodeID        string
	nodeIPAddress string

	// OS Specific
	kubeCtlPath           string
	rebootCommand         string
	sentinelExistsCommand string
	baseCommand           string

	// credentials
	powershellUserName string
	powershellPassword string

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

	rootCmd.PersistentFlags().DurationVar(&period, "period", time.Minute*60,
		"reboot check period")
	rootCmd.PersistentFlags().StringVar(&dsNamespace, "ds-namespace", "kube-system",
		"namespace containing daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&dsName, "ds-name", "kured",
		"name of daemonset on which to place lock")
	rootCmd.PersistentFlags().StringVar(&lockAnnotation, "lock-annotation", "weave.works/kured-node-lock",
		"annotation in which to record locking node")
	rootCmd.PersistentFlags().StringVar(&prometheusURL, "prometheus-url", "",
		"Prometheus instance to probe for active alerts")
	rootCmd.PersistentFlags().Var(&regexpValue{&alertFilter}, "alert-filter-regexp",
		"alert names to ignore when checking for active alerts")
	rootCmd.PersistentFlags().StringVar(&rebootSentinel, "reboot-sentinel", "/var/run/reboot-required",
		"path to file whose existence signals need to reboot")

	rootCmd.PersistentFlags().StringVar(&slackHookURL, "slack-hook-url", "",
		"slack hook URL for reboot notfications")
	rootCmd.PersistentFlags().StringVar(&slackUsername, "slack-username", "kured",
		"slack username for reboot notfications")

	rootCmd.PersistentFlags().StringArrayVar(&podSelectors, "blocking-pod-selector", nil,
		"label selector identifying pods whose presence should prevent reboots")

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func configureOsVariables() {
	if isWindows() {
		configureWindowsVariables()
		trustHost()
	} else {
		configureLinuxVariables()
	}
}

func root(cmd *cobra.Command, args []string) {
	log.Infof("Kubernetes Reboot Daemon: %s", version)
	log.Infof("Kubernetes Runtime Operating System: %s", runtime.GOOS)

	nodeID = os.Getenv("KURED_NODE_ID")
	if nodeID == "" {
		log.Fatal("KURED_NODE_ID environment variable required")
	}

	log.Infof("Node ID: %s", nodeID)
	log.Infof("Lock Annotation: %s/%s:%s", dsNamespace, dsName, lockAnnotation)
	log.Infof("Reboot Sentinel: %s every %v", rebootSentinel, period)
	log.Infof("Blocking Pod Selectors: %v", podSelectors)

	configureOsVariables()

	go rebootAsRequired()
	go maintainRebootRequiredMetric()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func configureLinuxVariables() {
	kubeCtlPath = "/usr/bin/kubectl"
	baseCommand = "/usr/bin/nsenter"
	// Relies on hostPID:true and privileged:true to enter host mount space
	rebootCommand = "-m/proc/1/ns/mnt /bin/systemctl reboot"
	// Relies on hostPID:true and privileged:true to enter host mount space
	sentinelExistsCommand = "-m/proc/1/ns/mnt -- /usr/bin/test -f"
}

func configureWindowsVariables() {
	powershellUserName = os.Getenv("KURED_POWERSHELL_USER_NAME")
	if powershellUserName == "" {
		log.Fatal("KURED_POWERSHELL_USER_NAME environment variable required")
	}

	powershellPassword = os.Getenv("KURED_POWERSHELL_PASSWORD")
	if powershellPassword == "" {
		log.Fatal("KURED_POWERSHELL_PASSWORD environment variable required")
	}

	nodeIPAddress = os.Getenv("KURED_NODE_IP_ADDRESS")
	if nodeIPAddress == "" {
		log.Fatal("KURED_NODE_IP_ADDRESS environment variable required")
	}

	kubeCtlPath = "C:\\k\\kubectl.exe"
	baseCommand = "powershell"
	rebootCommand = createPowerShellCommandString("shutdown /r /t 60 /c \"kured forcing reboot due to pending Windows updates\"")
	sentinelExistsCommand = createPowerShellCommandString("REG QUERY \"HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\WindowsUpdate\\Auto Update\\RebootRequired\"")
}

func trustHost() {
	trustCommand := newCommand("powershell", "Set-Item", "wsman:\\localhost\\client\\trustedhosts", nodeIPAddress, "-Force")
	if err := trustCommand.Run(); err != nil {
		// Cannot continue further as we can't trust the windows host
		log.Fatalf("Error trusting host %s: %v", nodeIPAddress, err)
	}
}

func createPowerShellCommandString(scriptBlock string) string {
	return fmt.Sprintf("$userName = \"%s\" ; $pwd = ConvertTo-SecureString \"%s\" -AsPlainText -Force; $credential = New-Object PSCredential $userName, $pwd; Invoke-Command -ComputerName %s -ScriptBlock { %s } -Credential $credential", powershellUserName, powershellPassword, nodeIPAddress, scriptBlock)
}

func drain() {
	log.Infof("Draining node %s", nodeID)

	if slackHookURL != "" {
		if err := slack.NotifyDrain(slackHookURL, slackUsername, nodeID); err != nil {
			log.Warnf("Error notifying slack: %v", err)
		}
	}

	drainCmd := newCommand(kubeCtlPath, "drain", "--ignore-daemonsets", "--delete-local-data", "--force", nodeID)

	if err := drainCmd.Run(); err != nil {
		log.Fatalf("Error invoking drain command: %v", err)
	}
}

func uncordon() {
	log.Infof("Uncordoning node %s", nodeID)
	uncordonCmd := newCommand(kubeCtlPath, "uncordon", nodeID)
	if err := uncordonCmd.Run(); err != nil {
		log.Fatalf("Error invoking uncordon command: %v", err)
	}
}

func commandReboot() {
	log.Infof("Commanding reboot")

	if slackHookURL != "" {
		if err := slack.NotifyReboot(slackHookURL, slackUsername, nodeID); err != nil {
			log.Warnf("Error notifying slack: %v", err)
		}
	}

	rebootCmd := newCommand(baseCommand, rebootCommand)

	if err := rebootCmd.Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}

func maintainRebootRequiredMetric() {
	for {
		if sentinelExists() {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(1)
		} else {
			rebootRequiredGauge.WithLabelValues(nodeID).Set(0)
		}
		time.Sleep(time.Minute)
	}
}

func rebootAsRequired() {
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
			uncordon()
		}
		release(lock)
	}

	source := rand.NewSource(time.Now().UnixNano())
	tick := delaytick.New(source, period)
	for _ = range tick {
		if rebootRequired() && !rebootBlocked(client) {
			node, err := client.CoreV1().Nodes().Get(nodeID, metav1.GetOptions{})
			if err != nil {
				log.Fatal(err)
			}
			nodeMeta.Unschedulable = node.Spec.Unschedulable

			if acquire(lock, &nodeMeta) {
				if !nodeMeta.Unschedulable {
					drain()
				}
				commandReboot()
				for {
					log.Infof("Waiting for reboot")
					time.Sleep(time.Minute)
				}
			}
		}
	}
}

func rebootRequired() bool {
	if sentinelExists() {
		log.Infof("Reboot required")
		return true
	} else {
		log.Infof("Reboot not required")
		return false
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

func rebootBlocked(client *kubernetes.Clientset) bool {
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

	fieldSelector := fmt.Sprintf("spenodeName=%s", nodeID)
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

func sentinelExists() bool {
	var sentinelCmd *exec.Cmd
	if isWindows() {
		sentinelCmd = newCommand(baseCommand, sentinelExistsCommand)
	} else {
		sentinelCmd = newCommand(baseCommand, sentinelExistsCommand, rebootSentinel)
	}

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
