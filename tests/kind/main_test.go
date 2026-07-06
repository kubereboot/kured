package kind

import (
	"bytes"
	"fmt"
	"math/rand"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

const (
	kuredDevImage string = "kured:dev"
)

// KindTest cluster deployed by each TestMain function, prepared to run a given test scenario.
type KindTest struct {
	kindConfigPath  string
	clusterName     string
	timeout         time.Duration
	deployManifests []string
	localImages     []string
	logsDir         string
	logBuffer       bytes.Buffer
	testInstance    *testing.T // Maybe move this to testing.TB
}

func (k *KindTest) Write(p []byte) (n int, err error) {
	k.testInstance.Helper()
	k.logBuffer.Write(p)
	return len(p), nil
}

func (k *KindTest) FlushLog() {
	k.testInstance.Helper()
	k.testInstance.Log(k.logBuffer.String())
	k.logBuffer.Reset()
}

func (k *KindTest) RunCmd(cmdDetails ...string) error {
	cmd := exec.Command(cmdDetails[0], cmdDetails[1:]...)
	// by making KindTest a Writer, we can simply wire k to logs
	// writing to k will write to proper logs.
	cmd.Stdout = k
	cmd.Stderr = k

	err := cmd.Run()
	if err != nil {
		return err
	}
	return nil
}

// Option that can be passed to the NewKind function in order to change the configuration
// of the test cluster
type Option func(k *KindTest)

// Deploy can be passed to NewKind to deploy extra components, in addition to the base deployment.
func Deploy(manifest string) Option {
	return func(k *KindTest) {
		k.deployManifests = append(k.deployManifests, manifest)
	}
}

// ExportLogs can be passed to NewKind to specify the folder where the kubernetes logs will be exported after the tests.
func ExportLogs(folder string) Option {
	return func(k *KindTest) {
		k.logsDir = folder
	}
}

// Timeout for long-running operations (e.g. deployments, readiness probes...)
func Timeout(t time.Duration) Option {
	return func(k *KindTest) {
		k.timeout = t
	}
}

// LocalImage is passed to NewKind to allow loading a local Docker image into the cluster
func LocalImage(nameTag string) Option {
	return func(k *KindTest) {
		k.localImages = append(k.localImages, nameTag)
	}
}

// NewKind creates a kind cluster given a name and set of Option instances.
func NewKindTester(kindClusterName string, filePath string, t *testing.T, options ...Option) *KindTest {

	k := &KindTest{
		clusterName:    kindClusterName,
		timeout:        10 * time.Minute,
		kindConfigPath: filePath,
		testInstance:   t,
	}
	for _, option := range options {
		option(k)
	}
	return k
}

// Prepare the kind cluster.
func (k *KindTest) Create() error {
	err := k.RunCmd("kind", "create", "cluster", "--name", k.clusterName, "--config", k.kindConfigPath)

	if err != nil {
		return fmt.Errorf("failed to create cluster: %v", err)
	}

	for _, img := range k.localImages {
		if err := k.RunCmd("kind", "load", "docker-image", "--name", k.clusterName, img); err != nil {
			return fmt.Errorf("failed to load image: %v", err)
		}
	}
	for _, mf := range k.deployManifests {
		kubectlContext := fmt.Sprintf("kind-%v", k.clusterName)
		if err := k.RunCmd("kubectl", "--context", kubectlContext, "apply", "-f", mf); err != nil {
			return fmt.Errorf("failed to deploy manifest: %v", err)
		}
	}
	return nil
}

func (k *KindTest) Destroy() error {
	if k.logsDir != "" {
		if err := k.RunCmd("kind", "export", "logs", k.logsDir, "--name", k.clusterName); err != nil {
			return fmt.Errorf("failed to export logs: %v. will not teardown", err)
		}
	}

	if err := k.RunCmd("kind", "delete", "cluster", "--name", k.clusterName); err != nil {
		return fmt.Errorf("failed to destroy cluster: %v", err)
	}
	return nil
}

const (
	currentK8sClusterConfig  = "../../.github/kind-cluster-current.yaml"
	previousK8sClusterConfig = "../../.github/kind-cluster-previous.yaml"
	rebootSentinelsScript    = "testfiles/create-reboot-sentinels.sh"
	coordinatedRebootScript  = "testfiles/follow-coordinated-reboot.sh"
)

type e2eScenario struct {
	clusterNamePrefix string
	clusterConfigFile string
	manifest          string
	setupScript       string
	testScript        string
	failureMessage    string
}

func runE2E(t *testing.T, scenario e2eScenario) {
	t.Helper()
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	randomInt := strconv.Itoa(rand.Intn(100))
	kindClusterName := fmt.Sprintf("%s-%s", scenario.clusterNamePrefix, randomInt)
	kindContext := fmt.Sprintf("kind-%v", kindClusterName)

	k := NewKindTester(kindClusterName, scenario.clusterConfigFile, t, LocalImage(kuredDevImage), Deploy("../../kured-rbac.yaml"), Deploy(scenario.manifest))
	defer k.FlushLog()

	err := k.Create()
	if err != nil {
		t.Fatalf("Error creating cluster %v", err)
	}
	defer func(k *KindTest) {
		err := k.Destroy()
		if err != nil {
			t.Fatalf("Error destroying cluster %v", err)
		}
	}(k)

	k.Write([]byte("Now running e2e tests"))

	if scenario.setupScript != "" {
		if err := k.RunCmd("bash", scenario.setupScript, kindContext); err != nil {
			t.Fatalf("failed to run setup script %s: %v", scenario.setupScript, err)
		}
	}

	if err := k.RunCmd("bash", scenario.testScript, kindContext); err != nil {
		t.Fatalf("%s: %v", scenario.failureMessage, err)
	}
}

func TestRebootCommandCurrentk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-command-current",
		clusterConfigFile: currentK8sClusterConfig,
		manifest:          "testfiles/kured-ds.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestRebootCommandPreviousk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-command-previous",
		clusterConfigFile: previousK8sClusterConfig,
		manifest:          "testfiles/kured-ds.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestRebootSignalCurrentk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-signal-current",
		clusterConfigFile: currentK8sClusterConfig,
		manifest:          "testfiles/kured-ds-signal.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestRebootSignalPreviousk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-signal-previous",
		clusterConfigFile: previousK8sClusterConfig,
		manifest:          "testfiles/kured-ds-signal.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestConcurrentRebootCommandCurrentk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-concurrentcommand-current",
		clusterConfigFile: currentK8sClusterConfig,
		manifest:          "testfiles/kured-ds-concurrent-command.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestConcurrentRebootCommandPreviousk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-concurrentcommand-previous",
		clusterConfigFile: previousK8sClusterConfig,
		manifest:          "testfiles/kured-ds-concurrent-command.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestConcurrentRebootSignalCurrentk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-concurrentsignal-current",
		clusterConfigFile: currentK8sClusterConfig,
		manifest:          "testfiles/kured-ds-concurrent-signal.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestConcurrentRebootSignalPreviousk8sRelease(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-concurrentsignal-previous",
		clusterConfigFile: previousK8sClusterConfig,
		manifest:          "testfiles/kured-ds-concurrent-signal.yaml",
		setupScript:       rebootSentinelsScript,
		testScript:        coordinatedRebootScript,
		failureMessage:    "failed to follow reboot",
	})
}

func TestCordonningIsKeptWithoutConcurrency(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-cordon-without-concurrency",
		clusterConfigFile: currentK8sClusterConfig,
		manifest:          "testfiles/kured-ds-signal.yaml",
		testScript:        "testfiles/node-stays-as-cordonned.sh",
		failureMessage:    "node did not reboot in time",
	})
}

func TestCordonningIsKeptWithConcurrency(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-cordon-with-concurrency",
		clusterConfigFile: currentK8sClusterConfig,
		manifest:          "testfiles/kured-ds-concurrent-signal.yaml",
		testScript:        "testfiles/node-stays-as-cordonned.sh",
		failureMessage:    "node did not reboot in time",
	})
}

func TestRebootBlockedPodblocker(t *testing.T) {
	runE2E(t, e2eScenario{
		clusterNamePrefix: "kured-e2e-cordon-podblocker",
		clusterConfigFile: currentK8sClusterConfig,
		manifest:          "testfiles/kured-ds-podblocker.yaml",
		testScript:        "testfiles/podblocker.sh",
		failureMessage:    "node blocker test did not succeed",
	})
}
