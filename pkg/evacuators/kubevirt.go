package evacuators

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	log "github.com/sirupsen/logrus"
)

// KubeVirtEvacuator implements Evacuator interface, managing KubeVirt instances
type KubeVirtEvacuator struct {
	client *kubernetes.Clientset // Kubernetes client object inherited from the caller
	nodeID string                // Kubernetes Node ID
	errors []error               // Errors created by the evacuation threads

	sleepingTime   time.Duration // Time in seconds between each VM state change
	timeoutCounter int           // Number of times checking each VM state change

	mutex sync.Mutex // Used to start non-threadsafe commands
}

// NewKubeVirtEvacuator is the constructor
func NewKubeVirtEvacuator(nodeID string, client *kubernetes.Clientset) (*KubeVirtEvacuator, error) {
	var result KubeVirtEvacuator
	var err error

	if client == nil {
		err = fmt.Errorf("NewKubeVirtEvacuator: the given clientset is nil")
	}

	if len(nodeID) == 0 {
		err = fmt.Errorf("NewKubeVirtEvacuator: the given nodeID is empty")
	}

	result.nodeID = nodeID
	result.client = client
	result.sleepingTime = 30
	result.timeoutCounter = 40

	return &result, err
}

// Evacuate start the live migration process of the hosted virtual instances
func (k *KubeVirtEvacuator) Evacuate() (err error) {
	log.Infof("Evacuate: migration configuration is %v retries every %v", k.timeoutCounter, k.sleepingTime*time.Second)

	vms, err := k.getVMRunningOnNode()

	if err == nil {
		k.startAsyncEvacuation(vms)

		for {
			if k.timeoutCounter == 0 {
				err = fmt.Errorf("Evacuate: timeout exceeded")
				break
			}

			log.Infof("EvacuateVM: %v retries left. %v remaining instances on the node", k.timeoutCounter, vms.Size())

			vms, err = k.getVMRunningOnNode()
			if err != nil {
				err = fmt.Errorf("%v errors occured", len(k.errors))
				break
			}

			if vms.Size() == 0 {
				log.Info("Evacuate: Completed.")
				break
			}

			k.countDown()
		}
	}

	return err
}

// startAsyncEvacuation starts one evacuateVM fonction per VM
func (k *KubeVirtEvacuator) startAsyncEvacuation(vms *v1.PodList) {
	for _, vm := range vms.Items {
		go k.evacuateVM(&vm)
	}
}

// countDown counts down the timer
func (k *KubeVirtEvacuator) countDown() {
	time.Sleep(k.sleepingTime * time.Second)
	k.timeoutCounter = k.timeoutCounter - 1
}

// getVMRunningOnNode gets the virt-launcher pods running on the node
func (k *KubeVirtEvacuator) getVMRunningOnNode() (*v1.PodList, error) {
	labelSelector := "kubevirt.io=virt-launcher"
	fieldSelector := fmt.Sprintf("spec.nodeName=%s", k.nodeID)

	return k.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
		FieldSelector: fieldSelector,
	})
}

// evacuateVM starts and monitors the migration of the given virtual instance
func (k *KubeVirtEvacuator) evacuateVM(vm *v1.Pod) {
	var newNode string
	var err error

	vmName := vm.Labels["kubevirt.io/vm"]

	if len(vmName) > 2 {
		logPrefix := fmt.Sprintf("evacuateVM: %s (ns: %s, pod %s).", vmName, vm.Namespace, vm.Name)
		shellCommand := exec.Command("/usr/bin/virtctl", "migrate", vmName, "-n", vm.Namespace)

		log.Infof("%s Evacuating from %s", logPrefix, k.nodeID)

		k.execCommand(shellCommand)
		if err != nil {
			err = fmt.Errorf("%s %v", logPrefix, err)
		} else {
			for {
				newNode, err = k.getNodeOfVM(vmName)
				if err != nil {
					break
				}

				if k.checkMigrationCompletion(logPrefix, newNode) {
					time.Sleep(k.sleepingTime * time.Second)
				} else {
					break
				}
			}
		}

		k.appendError(err)
	} else {
		log.Infof("given pod %s (ns %s) has an empty VM name. Skipping", vm.Name, vm.Namespace)
	}
}

// checkMigrationCompletion return true if the migration is completed
func (k *KubeVirtEvacuator) checkMigrationCompletion(logPrefix, newNode string) (result bool) {
	if k.nodeID == newNode {
		log.Infof("%s Still on %v", logPrefix, newNode)
	} else {
		log.Infof("%s Completed.", logPrefix)
		result = true
	}

	return result
}

// appendError append the given error to the internal errors array in a threadsafe way
func (k *KubeVirtEvacuator) appendError(err error) {
	if err != nil {
		k.mutex.Lock()
		k.errors = append(k.errors, err) // TODO: is append threadsafe?
		k.mutex.Unlock()
	}
}

// execCommand starts the given command in a threadsafe way
func (k *KubeVirtEvacuator) execCommand(command *exec.Cmd) (err error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	return command.Run()
}

// getNodeOfVM provides the node ID hosting the given virtual instance
func (k *KubeVirtEvacuator) getNodeOfVM(vmName string) (result string, err error) {
	var podList *v1.PodList

	if len(vmName) == 0 {
		err = fmt.Errorf("getNodeOfVM: the given VM name is empty")
	} else {
		labelSelector := fmt.Sprintf("kubevirt.io/vm=%s", vmName)

		podList, err = k.client.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	}

	if err == nil && podList != nil {
		result = podList.Items[0].Spec.NodeName
	}

	return result, err
}
