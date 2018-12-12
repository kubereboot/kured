package daemonsetlock

import (
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type DaemonSetLock struct {
	client     *kubernetes.Clientset
	nodeID     string
	namespace  string
	name       string
	annotation string
}

// NodeMeta is used to remember information across reboots
type NodeMeta struct {
	Unschedulable bool      `json:"unschedulable"`
	Expires       time.Time `json:"expires"`
}

type lockAnnotationValue struct {
	NodeID   string    `json:"nodeID"`
	Metadata *NodeMeta `json:"metadata,omitempty"`
}

func New(client *kubernetes.Clientset, nodeID, namespace, name, annotation string) *DaemonSetLock {
	return &DaemonSetLock{client, nodeID, namespace, name, annotation}
}

func (dsl *DaemonSetLock) Acquire(metadata *NodeMeta) (acquired bool, owner string, err error) {
	for {
		ds, err := dsl.client.AppsV1().DaemonSets(dsl.namespace).Get(dsl.name, metav1.GetOptions{})
		if err != nil {
			return false, "", err
		}

		valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
		if exists {
			value := lockAnnotationValue{}
			if err := json.Unmarshal([]byte(valueString), &value); err != nil {
				return false, "", err
			}

			if value.Metadata.Expires.After(time.Now()) {
				return value.NodeID == dsl.nodeID, value.NodeID, nil
			}
		}

		if ds.ObjectMeta.Annotations == nil {
			ds.ObjectMeta.Annotations = make(map[string]string)
		}
		value := lockAnnotationValue{NodeID: dsl.nodeID, Metadata: metadata}
		valueBytes, err := json.Marshal(&value)
		if err != nil {
			return false, "", err
		}
		ds.ObjectMeta.Annotations[dsl.annotation] = string(valueBytes)

		_, err = dsl.client.AppsV1().DaemonSets(dsl.namespace).Update(ds)
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

func (dsl *DaemonSetLock) Test(metadata *NodeMeta) (holding bool, err error) {
	ds, err := dsl.client.AppsV1().DaemonSets(dsl.namespace).Get(dsl.name, metav1.GetOptions{})

	if err != nil {
		return false, err
	}

	valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
	if exists {
		value := lockAnnotationValue{Metadata: metadata}
		if err := json.Unmarshal([]byte(valueString), &value); err != nil {
			return false, err
		}
		return value.NodeID == dsl.nodeID, nil
	}

	return false, nil
}

func (dsl *DaemonSetLock) Release() error {
	for {
		ds, err := dsl.client.AppsV1().DaemonSets(dsl.namespace).Get(dsl.name, metav1.GetOptions{})
		if err != nil {
			return err
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

		_, err = dsl.client.AppsV1().DaemonSets(dsl.namespace).Update(ds)
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
