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

func TestE2EWithCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var kindClusterConfigs = []string{
		"previous",
		"current",
		"next",
	}
	// Iterate over each Kubernetes version
	for _, version := range kindClusterConfigs {
		version := version
		// Define a subtest for each combination
		t.Run(version, func(t *testing.T) {
			t.Parallel() // Allow tests to run in parallel

			randomInt := strconv.Itoa(rand.Intn(100))
			kindClusterName := fmt.Sprintf("kured-e2e-command-%v-%v", version, randomInt)
			kindClusterConfigFile := fmt.Sprintf("../../.github/kind-cluster-%v.yaml", version)
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			k := NewKindTester(kindClusterName, kindClusterConfigFile, t, LocalImage(kuredDevImage), Deploy("../../kured-rbac.yaml"), Deploy("testfiles/kured-ds.yaml"))
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

			if err := k.RunCmd("bash", "testfiles/create-reboot-sentinels.sh", kindContext); err != nil {
				t.Fatalf("failed to create sentinels: %v", err)
			}

			if err := k.RunCmd("bash", "testfiles/follow-coordinated-reboot.sh", kindContext); err != nil {
				t.Fatalf("failed to follow reboot: %v", err)
			}
		})
	}
}

func TestE2EWithSignal(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var kindClusterConfigs = []string{
		"previous",
		"current",
		"next",
	}
	// Iterate over each Kubernetes version
	for _, version := range kindClusterConfigs {
		version := version
		// Define a subtest for each combination
		t.Run(version, func(t *testing.T) {
			t.Parallel() // Allow tests to run in parallel

			randomInt := strconv.Itoa(rand.Intn(100))
			kindClusterName := fmt.Sprintf("kured-e2e-signal-%v-%v", version, randomInt)
			kindClusterConfigFile := fmt.Sprintf("../../.github/kind-cluster-%v.yaml", version)
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			k := NewKindTester(kindClusterName, kindClusterConfigFile, t, LocalImage(kuredDevImage), Deploy("../../kured-rbac.yaml"), Deploy("testfiles/kured-ds-signal.yaml"))
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

			if err := k.RunCmd("bash", "testfiles/create-reboot-sentinels.sh", kindContext); err != nil {
				t.Fatalf("failed to create sentinels: %v", err)
			}

			if err := k.RunCmd("bash", "testfiles/follow-coordinated-reboot.sh", kindContext); err != nil {
				t.Fatalf("failed to follow reboot: %v", err)
			}
		})
	}
}

func TestE2EConcurrentWithCommand(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var kindClusterConfigs = []string{
		"previous",
		"current",
		"next",
	}
	// Iterate over each Kubernetes version
	for _, version := range kindClusterConfigs {
		version := version
		// Define a subtest for each combination
		t.Run(version, func(t *testing.T) {
			t.Parallel() // Allow tests to run in parallel

			randomInt := strconv.Itoa(rand.Intn(100))
			kindClusterName := fmt.Sprintf("kured-e2e-concurrentcommand-%v-%v", version, randomInt)
			kindClusterConfigFile := fmt.Sprintf("../../.github/kind-cluster-%v.yaml", version)
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			k := NewKindTester(kindClusterName, kindClusterConfigFile, t, LocalImage(kuredDevImage), Deploy("../../kured-rbac.yaml"), Deploy("testfiles/kured-ds-concurrent-command.yaml"))
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

			if err := k.RunCmd("bash", "testfiles/create-reboot-sentinels.sh", kindContext); err != nil {
				t.Fatalf("failed to create sentinels: %v", err)
			}

			if err := k.RunCmd("bash", "testfiles/follow-coordinated-reboot.sh", kindContext); err != nil {
				t.Fatalf("failed to follow reboot: %v", err)
			}
		})
	}
}

func TestE2EConcurrentWithSignal(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var kindClusterConfigs = []string{
		"previous",
		"current",
		"next",
	}
	// Iterate over each Kubernetes version
	for _, version := range kindClusterConfigs {
		version := version
		// Define a subtest for each combination
		t.Run(version, func(t *testing.T) {
			t.Parallel() // Allow tests to run in parallel

			randomInt := strconv.Itoa(rand.Intn(100))
			kindClusterName := fmt.Sprintf("kured-e2e-concurrentsignal-%v-%v", version, randomInt)
			kindClusterConfigFile := fmt.Sprintf("../../.github/kind-cluster-%v.yaml", version)
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			k := NewKindTester(kindClusterName, kindClusterConfigFile, t, LocalImage(kuredDevImage), Deploy("../../kured-rbac.yaml"), Deploy("testfiles/kured-ds-concurrent-signal.yaml"))
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

			if err := k.RunCmd("bash", "testfiles/create-reboot-sentinels.sh", kindContext); err != nil {
				t.Fatalf("failed to create sentinels: %v", err)
			}

			if err := k.RunCmd("bash", "testfiles/follow-coordinated-reboot.sh", kindContext); err != nil {
				t.Fatalf("failed to follow reboot: %v", err)
			}
		})
	}
}

func TestCordonningIsKept(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var kindClusterConfigs = []string{
		"concurrency1",
		"concurrency2",
	}
	// Iterate over each test variant
	for _, variant := range kindClusterConfigs {
		variant := variant
		// Define a subtest for each combination
		t.Run(variant, func(t *testing.T) {
			t.Parallel() // Allow tests to run in parallel

			randomInt := strconv.Itoa(rand.Intn(100))
			kindClusterName := fmt.Sprintf("kured-e2e-cordon-%v-%v", variant, randomInt)
			kindClusterConfigFile := "../../.github/kind-cluster-next.yaml"
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			var manifest string
			if variant == "concurrency1" {
				manifest = "testfiles/kured-ds-signal.yaml"
			} else {
				manifest = "testfiles/kured-ds-concurrent-signal.yaml"
			}
			k := NewKindTester(kindClusterName, kindClusterConfigFile, t, LocalImage(kuredDevImage), Deploy("../../kured-rbac.yaml"), Deploy(manifest))
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

			if err := k.RunCmd("bash", "testfiles/node-stays-as-cordonned.sh", kindContext); err != nil {
				t.Fatalf("node did not reboot in time: %v", err)
			}
		})
	}
}
func TestE2EBlocker(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	var kindClusterConfigs = []string{
		"podblocker",
	}
	// Iterate over each variant of the test
	for _, variant := range kindClusterConfigs {
		variant := variant
		// Define a subtest for each combination
		t.Run(variant, func(t *testing.T) {
			t.Parallel() // Allow tests to run in parallel

			randomInt := strconv.Itoa(rand.Intn(100))
			kindClusterName := fmt.Sprintf("kured-e2e-cordon-%v-%v", variant, randomInt)
			kindClusterConfigFile := "../../.github/kind-cluster-next.yaml"
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			k := NewKindTester(kindClusterName, kindClusterConfigFile, t, LocalImage(kuredDevImage), Deploy("../../kured-rbac.yaml"), Deploy(fmt.Sprintf("testfiles/kured-ds-%v.yaml", variant)))
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

			if err := k.RunCmd("bash", fmt.Sprintf("testfiles/%v.sh", variant), kindContext); err != nil {
				t.Fatalf("node blocker test did not succeed: %v", err)
			}
		})
	}
}
