package daemonsetlock

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

const (
	k8sAPICallRetrySleep   = 5 * time.Second // How much time to wait in between retrying a k8s API call
	k8sAPICallRetryTimeout = 5 * time.Minute // How long to wait until we determine that the k8s API is definitively unavailable
)

// DaemonSetLock holds all necessary information to do actions
// on the kured ds which holds lock info through annotations.
type DaemonSetLock struct {
	client     *kubernetes.Clientset
	nodeID     string
	namespace  string
	name       string
	annotation string
}

type lockAnnotationValue struct {
	NodeID   string        `json:"nodeID"`
	Metadata interface{}   `json:"metadata,omitempty"`
	Created  time.Time     `json:"created"`
	TTL      time.Duration `json:"TTL"`
}

// New creates a daemonsetLock object containing the necessary data for follow up k8s requests
func New(client *kubernetes.Clientset, nodeID, namespace, name, annotation string) *DaemonSetLock {
	return &DaemonSetLock{client, nodeID, namespace, name, annotation}
}

// Acquire attempts to annotate the kured daemonset with lock info from instantiated DaemonSetLock using client-go
func (dsl *DaemonSetLock) Acquire(metadata interface{}, TTL time.Duration) (acquired bool, owner string, err error) {
	for {
		var ds *v1.DaemonSet
		var lastError error
		err := wait.PollImmediate(k8sAPICallRetrySleep, k8sAPICallRetryTimeout, func() (bool, error) {
			ctx, cancel := context.WithTimeout(context.Background(), k8sAPICallRetryTimeout)
			defer cancel()
			if ds, lastError = dsl.client.AppsV1().DaemonSets(dsl.namespace).Get(ctx, dsl.name, metav1.GetOptions{}); lastError != nil {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return false, "", fmt.Errorf("Timed out trying to get daemonset %s in namespace %s: %v", dsl.name, dsl.namespace, lastError)
		}

		valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
		if exists {
			value := lockAnnotationValue{}
			if err := json.Unmarshal([]byte(valueString), &value); err != nil {
				return false, "", err
			}

			if !ttlExpired(value.Created, value.TTL) {
				return value.NodeID == dsl.nodeID, value.NodeID, nil
			}
		}

		if ds.ObjectMeta.Annotations == nil {
			ds.ObjectMeta.Annotations = make(map[string]string)
		}
		value := lockAnnotationValue{NodeID: dsl.nodeID, Metadata: metadata, Created: time.Now().UTC(), TTL: TTL}
		valueBytes, err := json.Marshal(&value)
		if err != nil {
			return false, "", err
		}
		ds.ObjectMeta.Annotations[dsl.annotation] = string(valueBytes)

		_, err = dsl.client.AppsV1().DaemonSets(dsl.namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		if err != nil {
			if se, ok := err.(*errors.StatusError); ok && se.ErrStatus.Reason == metav1.StatusReasonConflict {
				// Something else updated the resource between us reading and writing - try again soon
				time.Sleep(time.Second)
				continue
			} else {
				return false, "", err
			}
		}
		return true, dsl.nodeID, nil
	}
}

// Test attempts to check the kured daemonset lock status (existence, expiry) from instantiated DaemonSetLock using client-go
func (dsl *DaemonSetLock) Test(metadata interface{}) (holding bool, err error) {
	var ds *v1.DaemonSet
	var lastError error
	err = wait.PollImmediate(k8sAPICallRetrySleep, k8sAPICallRetryTimeout, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), k8sAPICallRetryTimeout)
		defer cancel()
		if ds, lastError = dsl.client.AppsV1().DaemonSets(dsl.namespace).Get(ctx, dsl.name, metav1.GetOptions{}); lastError != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return false, fmt.Errorf("Timed out trying to get daemonset %s in namespace %s: %v", dsl.name, dsl.namespace, lastError)
	}

	valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
	if exists {
		value := lockAnnotationValue{Metadata: metadata}
		if err := json.Unmarshal([]byte(valueString), &value); err != nil {
			return false, err
		}

		if !ttlExpired(value.Created, value.TTL) {
			return value.NodeID == dsl.nodeID, nil
		}
	}

	return false, nil
}

// Release attempts to remove the lock data from the kured ds annotations using client-go
func (dsl *DaemonSetLock) Release() error {
	for {
		var ds *v1.DaemonSet
		var lastError error
		err := wait.PollImmediate(k8sAPICallRetrySleep, k8sAPICallRetryTimeout, func() (bool, error) {
			ctx, cancel := context.WithTimeout(context.Background(), k8sAPICallRetryTimeout)
			defer cancel()
			if ds, lastError = dsl.client.AppsV1().DaemonSets(dsl.namespace).Get(ctx, dsl.name, metav1.GetOptions{}); lastError != nil {
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return fmt.Errorf("Timed out trying to get daemonset %s in namespace %s: %v", dsl.name, dsl.namespace, lastError)
		}

		valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
		if exists {
			value := lockAnnotationValue{}
			if err := json.Unmarshal([]byte(valueString), &value); err != nil {
				return err
			}

			if value.NodeID != dsl.nodeID {
				return fmt.Errorf("Not lock holder: %v", value.NodeID)
			}
		} else {
			return fmt.Errorf("Lock not held")
		}

		delete(ds.ObjectMeta.Annotations, dsl.annotation)

		_, err = dsl.client.AppsV1().DaemonSets(dsl.namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		if err != nil {
			if se, ok := err.(*errors.StatusError); ok && se.ErrStatus.Reason == metav1.StatusReasonConflict {
				// Something else updated the resource between us reading and writing - try again soon
				time.Sleep(time.Second)
				continue
			} else {
				return err
			}
		}
		return nil
	}
}

func ttlExpired(created time.Time, ttl time.Duration) bool {
	if ttl > 0 && time.Since(created) >= ttl {
		return true
	}
	return false
}
