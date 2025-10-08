package e2e

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	kindManifestFileName string = "kind-manifest.yaml"
)

var KindVersionImages = map[string]string{
	"previous": "kindest/node:v1.32.8",
	"current":  "kindest/node:v1.33.4",
	"next":     "kindest/node:v1.34.0",
}

// KindManifest is a struct that represents a kind manifest. This is used to generate the kind manifest
// without adding a hard dependency on kind or sigs.k8s.io/yaml.
type KindManifest struct {
	Kind       string `json:"kind"`
	ApiVersion string `json:"apiVersion"`
	Nodes      []struct {
		Role  string `json:"role"`
		Image string `json:"image"`
	} `json:"nodes"`
}

func NewKindManifest(workerNodesCount int, cpNodesCount int, image string) *KindManifest {
	manifest := &KindManifest{
		Kind:       "Cluster",
		ApiVersion: "kind.x-k8s.io/v1alpha4",
		Nodes: []struct {
			Role  string `json:"role"`
			Image string `json:"image"`
		}{},
	}

	// Add control-plane nodes
	for i := 0; i < cpNodesCount; i++ {
		manifest.Nodes = append(manifest.Nodes, struct {
			Role  string `json:"role"`
			Image string `json:"image"`
		}{
			Role:  "control-plane",
			Image: image,
		})
	}

	// Add worker nodes
	for i := 0; i < workerNodesCount; i++ {
		manifest.Nodes = append(manifest.Nodes, struct {
			Role  string `json:"role"`
			Image string `json:"image"`
		}{
			Role:  "worker",
			Image: image,
		})
	}

	return manifest
}

// ToJSON returns the JSON encoding of the KindManifest.
func (m *KindManifest) ToJSON() (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (m *KindManifest) ToDisk(path string) error {
	jsonStr, err := m.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to convert manifest to JSON: %v", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("WARNING: Failed to close file: %v", err)
		}
	}(file)

	_, err = file.WriteString(jsonStr)
	if err != nil {
		return err
	}
	return nil
}

// NewKindTester creates a kind cluster for e2e test, given a name and set of test Option instances.
func NewKindTester(t *testing.T, clusterImage string, options ...Option) *E2ETest {
	if t != nil {
		t.Helper()
	} else {
		log.Fatalln("t is nil, this is not allowed. Please use this from a test.")
	}

	dir, err := os.MkdirTemp("", fmt.Sprintf("%s-%s-*", artifactFolderPrefix, strings.ReplaceAll(t.Name(), "/", "-")))
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	k := &E2ETest{
		clusterName:    "kured",
		clusterImage:   clusterImage,
		timeout:        5 * time.Minute,
		testInstance:   t,
		artifactFolder: dir,
		keepFiles:      false,
		keepCluster:    false,
		cpNodes: &TestNodes{
			role:     "control-plane",
			quantity: 1,
		},
	}
	for _, option := range options {
		option(k)
	}
	return k
}

// Prepare the cluster for e2e testing.
// For kind, this includes:
// - creating the cluster
// - wait for the cluster to be ready
// - pushing the images
// - deploying the manifests
func (k *E2ETest) Prepare() error {
	if err := createKindCluster(k); err != nil {
		return err
	}

	if err := pushKindImages(k); err != nil {
		return err
	}

	return k.DeployManifests()
}

func createKindCluster(k *E2ETest) error {
	manifest := NewKindManifest(k.workerNodes.quantity, k.cpNodes.quantity, k.clusterImage)
	manifestFile := filepath.Join(k.artifactFolder, kindManifestFileName)

	if err := manifest.ToDisk(manifestFile); err != nil {
		return fmt.Errorf("failed to write kind manifest: %v", err)
	}

	// Ensure at least 5 minutes as timeout for cluster creation
	createTimeout := time.Duration(float64(5*time.Minute) * getPerformanceFactor())
	err := k.RunCmdWithTimeout(createTimeout, "kind", "create", "cluster", "--name", k.clusterName, "--config", manifestFile)
	if err != nil {
		return fmt.Errorf("failed to create kind cluster: %v", err)
	}
	return nil
}

func pushKindImages(k *E2ETest) error {
	amountOfImagesToPush := len(k.localImages) * (1 + k.workerNodes.quantity)
	imagePushTimeout := time.Duration(float64(30*time.Second) * float64(amountOfImagesToPush) * getPerformanceFactor())

	for _, img := range k.localImages {
		if err := k.RunCmdWithTimeout(imagePushTimeout, "kind", "load", "docker-image", "--name", k.clusterName, img); err != nil {
			return fmt.Errorf("failed to load image: %v", err)
		}
	}
	return nil
}
