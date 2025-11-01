package e2e

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

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
		// Define a subtest for each combination.
		// I prefer repeating myself with anonymous test functions to run here below than a global one with a switch statement.
		// If this becomes unreadable, we can always move to a switch statement, which would change the nodes, the manifests, and the test script.
		t.Run(version, func(t *testing.T) {
			t.Parallel() // Allow tests to run in parallel

			randomInt := strconv.Itoa(rand.Intn(100))
			kindClusterName := fmt.Sprintf("kured-e2e-command-%v-%v", version, randomInt)
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			k := NewKindTester(t, KindVersionImages[version],
				WithClusterName(kindClusterName),
				WithWorkerNodes(1),
				PreloadImages(kuredDevImage),
				Deploy("../../kured-rbac.yaml"),
			)

			defer k.FlushLog()

			kuredDSFile := filepath.Join(k.artifactFolder, kuredDSManifestFilename)

			pkds, err := PatchDaemonSet(NewKuredDaemonSet(), KuredDSCommandPatch)
			if err != nil {
				t.Fatalf("failed to patch DaemonSet: %v", err)
			}
			if err := SaveDaemonsetToDisk(pkds, kuredDSFile); err != nil {
				t.Fatalf("failed to save patched DaemonSet: %v", err)
			}
			Deploy(kuredDSFile)(k)

			if err := k.Prepare(); err != nil {
				t.Fatalf("Error creating cluster %v", err)
			}
			defer func(k *E2ETest) {
				err := k.Destroy()
				if err != nil {
					t.Fatalf("Error destroying cluster %v", err)
				}
			}(k)

			if err := k.RunCmdWithDefaultTimeout("bash", "testdata/create-reboot-sentinels.sh", kindContext); err != nil {
				t.Fatalf("failed to create sentinels: %v", err)
			}

			if err := k.RunCmdWithDefaultTimeout("bash", "testdata/follow-coordinated-reboot.sh", kindContext); err != nil {
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
			kindContext := fmt.Sprintf("kind-%v", kindClusterName)

			k := NewKindTester(t, KindVersionImages[version],
				WithClusterName(kindClusterName),
				WithWorkerNodes(1),
				PreloadImages(kuredDevImage),
				Deploy("../../kured-rbac.yaml"),
			)
			defer k.FlushLog()

			kuredDSFile := filepath.Join(k.artifactFolder, kuredDSManifestFilename)

			pkds, err := PatchDaemonSet(NewKuredDaemonSet(), KuredDSSignalPatch)
			if err != nil {
				t.Fatalf("failed to patch DaemonSet: %v", err)
			}
			if err := SaveDaemonsetToDisk(pkds, kuredDSFile); err != nil {
				t.Fatalf("failed to save patched DaemonSet: %v", err)
			}
			Deploy(kuredDSFile)(k)

			if err := k.Prepare(); err != nil {
				t.Fatalf("Error creating cluster %v", err)
			}
			defer func(k *E2ETest) {
				err := k.Destroy()
				if err != nil {
					t.Fatalf("Error destroying cluster %v", err)
				}
			}(k)

			if err := k.RunCmdWithDefaultTimeout("bash", "testdata/create-reboot-sentinels.sh", kindContext); err != nil {
				t.Fatalf("failed to create sentinels: %v", err)
			}

			if err := k.RunCmdWithDefaultTimeout("bash", "testdata/follow-coordinated-reboot.sh", kindContext); err != nil {
				t.Fatalf("failed to follow reboot: %v", err)
			}
		})
	}
}

func TestE2ECordoning(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	kindClusterName := fmt.Sprintf("kured-e2e-feature-cordoning-%v", strconv.Itoa(rand.Intn(100)))
	kindContext := fmt.Sprintf("kind-%v", kindClusterName)
	k := NewKindTester(t, KindVersionImages["next"],
		WithClusterName(kindClusterName),
		WithWorkerNodes(2), // one cordonned, one not cordonned
		PreloadImages(kuredDevImage),
		Deploy("../../kured-rbac.yaml"),
	)
	defer k.FlushLog()

	kuredDSFile := filepath.Join(k.artifactFolder, kuredDSManifestFilename)

	pkds, err := PatchDaemonSet(NewKuredDaemonSet(), KuredDSSignalPatch)
	if err != nil {
		t.Fatalf("failed to patch DaemonSet: %v", err)
	}
	if err := SaveDaemonsetToDisk(pkds, kuredDSFile); err != nil {
		t.Fatalf("failed to save patched DaemonSet: %v", err)
	}
	Deploy(kuredDSFile)(k)

	if err := k.Prepare(); err != nil {
		t.Fatalf("Error creating cluster %v", err)
	}

	defer func(k *E2ETest) {
		err := k.Destroy()
		if err != nil {
			t.Fatalf("Error destroying cluster %v", err)
		}
	}(k)

	if err := k.RunCmdWithDefaultTimeout("bash", "testdata/node-stays-as-cordonned.sh", kindContext); err != nil {
		t.Fatalf("node did not reboot in time: %v", err)
	}

}

func TestE2EConcurrency(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	kindClusterName := fmt.Sprintf("kured-e2e-feature-concurrency-%v", strconv.Itoa(rand.Intn(100)))
	kindContext := fmt.Sprintf("kind-%v", kindClusterName)
	k := NewKindTester(t, KindVersionImages["next"],
		WithClusterName(kindClusterName),
		WithWorkerNodes(3),
		Timeout(10*time.Minute), // Extending timeout to 10 minutes for this longer test (more nodes)
		PreloadImages(kuredDevImage),
		Deploy("../../kured-rbac.yaml"),
	)
	defer k.FlushLog()

	var concurrencyPatch = []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/spec/template/spec/containers/0/command/-",
			"value": "--concurrency=2",
		},
	}

	kuredDSFile := filepath.Join(k.artifactFolder, kuredDSManifestFilename)

	pkds, err := PatchDaemonSet(NewKuredDaemonSet(), KuredDSSignalPatch, concurrencyPatch)
	if err != nil {
		t.Fatalf("failed to patch DaemonSet: %v", err)
	}
	if err := SaveDaemonsetToDisk(pkds, kuredDSFile); err != nil {
		t.Fatalf("failed to save patched DaemonSet: %v", err)
	}
	Deploy(kuredDSFile)(k)

	if err := k.Prepare(); err != nil {
		t.Fatalf("Error creating cluster %v", err)
	}

	defer func(k *E2ETest) {
		err := k.Destroy()
		if err != nil {
			t.Fatalf("Error destroying cluster %v", err)
		}
	}(k)

	if err := k.RunCmdWithDefaultTimeout("bash", "testdata/create-reboot-sentinels.sh", kindContext); err != nil {
		t.Fatalf("failed to create sentinels: %v", err)
	}

	// TODO: Implement a real check that ensures the reboot is done in parallel
	if err := k.RunCmdWithDefaultTimeout("bash", "testdata/follow-coordinated-reboot.sh", kindContext, "3"); err != nil {
		t.Fatalf("failed to follow reboot: %v", err)
	}

}

func TestE2EPodBlocker(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	kindClusterName := fmt.Sprintf("kured-e2e-feature-podblocker-%v", strconv.Itoa(rand.Intn(100)))
	kindContext := fmt.Sprintf("kind-%v", kindClusterName)
	k := NewKindTester(t, KindVersionImages["next"],
		WithClusterName(kindClusterName),
		WithWorkerNodes(1),
		PreloadImages(kuredDevImage),
		Deploy("../../kured-rbac.yaml"),
	)
	defer k.FlushLog()

	kuredDSFile := filepath.Join(k.artifactFolder, kuredDSManifestFilename)

	var podSelectorPatch = []map[string]interface{}{
		{
			"op":    "add",
			"path":  "/spec/template/spec/containers/0/command/-",
			"value": "--blocking-pod-selector=app=blocker",
		},
	}
	pkds, err := PatchDaemonSet(NewKuredDaemonSet(), KuredDSSignalPatch, podSelectorPatch)
	if err != nil {
		t.Fatalf("failed to patch DaemonSet: %v", err)
	}
	if err := SaveDaemonsetToDisk(pkds, kuredDSFile); err != nil {
		t.Fatalf("failed to save patched DaemonSet: %v", err)
	}
	Deploy(kuredDSFile)(k)

	if err := k.Prepare(); err != nil {
		t.Fatalf("Error creating cluster %v", err)
	}

	defer func(k *E2ETest) {
		err := k.Destroy()
		if err != nil {
			t.Fatalf("Error destroying cluster %v", err)
		}
	}(k)

	if err := k.RunCmdWithDefaultTimeout("bash", "testdata/podblocker.sh", kindContext); err != nil {
		t.Fatalf("node blocker test did not succeed: %v", err)
	}

}
