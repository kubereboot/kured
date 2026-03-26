package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	kuredDevImage        string = "ghcr.io/kubereboot/kured:dev"
	artifactFolderPrefix string = "kured"
)

// Note: Generated test manifests will use the image specified by the KURED_TEST_IMAGE
// environment variable when set; otherwise they default to ghcr.io/kubereboot/kured:dev.

// getPerformanceFactor returns the multiplier for all timeouts from PERFORMANCE_FACTOR env var
func getPerformanceFactor() float64 {
	factor := os.Getenv("PERFORMANCE_FACTOR")
	if factor == "" {
		return 1.0
	}
	f, err := strconv.ParseFloat(factor, 64)
	if err != nil || f <= 0 {
		return 1.0
	}
	return f
}

type TestNodes struct {
	role     string
	quantity int
}

// E2ETest cluster deployed by each TestMain function, prepared to run a given test scenario.
// TODO: Move to a generic E2E test and interface + implement kind as first instance. Move towards the use of kubectx, to allow an EXTERNAL test.
type E2ETest struct {
	clusterName    string
	clusterImage   string        // contains the image name to use for all the nodes in the cluster
	timeout        time.Duration // timeout for the test itself, not the fixture
	manifests      []string      // list of manifests to deploy in the cluster
	localImages    []string      // list of local images to load into the cluster
	artifactFolder string        // contains test artifacts: manifests, logs, ...
	keepFiles      bool          // do not delete the artifacts folder after the test
	keepCluster    bool          // do not delete the kind cluster after the test
	logBuffer      bytes.Buffer  // buffer for logs for the test so they are not interleaved with the logs of other tests
	testInstance   *testing.T    // Maybe move this to testing.TB
	cpNodes        *TestNodes    // number of control plane nodes, usually 1
	workerNodes    *TestNodes
}

// Write implements io.Writer for logging into the test
// This allows parallel running while not messing up the logs
func (k *E2ETest) Write(p []byte) (n int, err error) {
	k.testInstance.Helper()
	k.logBuffer.Write(p)
	return len(p), nil
}

// Flush the buffer to the test log
func (k *E2ETest) FlushLog() {
	k.testInstance.Helper()
	k.testInstance.Log(k.logBuffer.String())
	k.logBuffer.Reset()
}

func (k *E2ETest) RunCmdWithDefaultTimeout(cmdDetails ...string) error {
	return k.RunCmdWithTimeout(k.timeout, cmdDetails...)
}

func (k *E2ETest) RunCmdWithTimeout(timeout time.Duration, cmdDetails ...string) error {
	k.testInstance.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdDetails[0], cmdDetails[1:]...)
	// by making E2ETest a Writer, we can simply wire k to logs
	// writing to k will write to proper logs.
	cmd.Stdout = k
	cmd.Stderr = k

	err := cmd.Run()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("command timed out after %v: %w", timeout, err)
		}
		return fmt.Errorf("command failed: %w", err)
	}
	return nil
}

// Option that can be passed to the NewKind function in order to change the configuration
// of the test cluster
type Option func(k *E2ETest)

// Deploy can be passed to NewKind to deploy extra components, in addition to the base deployment.
func Deploy(manifest string) Option {
	return func(k *E2ETest) {
		k.manifests = append(k.manifests, manifest)
	}
}

// Timeout is defining a timeout for the actual testing, not the fixture.
// Each test can have different durations (so we make this configurable per test),
// but we want the main kind create/push images/... (fixtures) to be independent of this value
func Timeout(t time.Duration) Option {
	return func(k *E2ETest) {
		k.timeout = time.Duration(float64(t) * getPerformanceFactor())
	}
}

// PreloadImages stores infos about the local images that will be pushed into the cluster
func PreloadImages(nameTag string) Option {
	return func(k *E2ETest) {
		k.localImages = append(k.localImages, nameTag)
	}
}

// WithClusterName sets the name of the cluster to create. This allows using different names than kured and making it easy to find clusters' artifacts.
func WithClusterName(name string) Option {
	return func(k *E2ETest) {
		k.clusterName = name
	}
}

// WithWorkerNodes sets the number of worker nodes for the cluster
func WithWorkerNodes(count int) Option {
	return func(k *E2ETest) {
		k.workerNodes = &TestNodes{
			role:     "worker",
			quantity: count,
		}
	}
}

// KeepArtifacts is a convenience function to keep the artifacts of the test.
// This is useful for debugging purposes. Add it to your test to keep the artifacts.
// You can decide to also keep the kind cluster running
func KeepArtifacts(keepCluster bool) Option {
	return func(k *E2ETest) {
		k.keepFiles = true
		k.keepCluster = keepCluster
	}
}

// TODO: Move to use generic Setup (and Deploy) interfaces, and implement kind as first instance.
func (k *E2ETest) DeployManifests() error {
	for _, mf := range k.manifests {
		kubectlContext := fmt.Sprintf("kind-%v", k.clusterName)
		if err := k.RunCmdWithDefaultTimeout("kubectl", "--context", kubectlContext, "apply", "-f", mf, "--wait=true"); err != nil {
			return fmt.Errorf("failed to deploy manifest: %v", err)
		}
	}
	return nil
}

// TODO: Move to use a generic Teardown interface, and implement kind as first instance.
func (k *E2ETest) Destroy() error {
	if k.testInstance == nil {
		return fmt.Errorf("testInstance is nil, cannot tear down cluster %s", k.clusterName)
	}
	k.testInstance.Helper()

	if !k.keepFiles && strings.Contains(k.artifactFolder, artifactFolderPrefix) {
		err := os.RemoveAll(k.artifactFolder)
		if err != nil {
			k.testInstance.Logf("WARNING: Failed to remove file artifacts: %v", err)
		}
	} else {
		if err := k.RunCmdWithDefaultTimeout("kind", "export", "logs", filepath.Join(k.artifactFolder, "logs"), "--name", k.clusterName); err != nil {
			k.testInstance.Logf("WARNING: Failed to export logs: %v (continuing with cleanup)", err)
		}
	}

	if !k.keepCluster {
		if err := k.RunCmdWithDefaultTimeout("kind", "delete", "cluster", "--name", k.clusterName); err != nil {
			return fmt.Errorf("failed to destroy cluster: %v", err)
		}
	}
	return nil
}
