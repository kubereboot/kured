package daemonsetlock

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
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

type Lock interface {
	Acquire(NodeMeta) (bool, error)
	Release() error
	GetNodeData() (bool, NodeMeta, error)
}

type GenericLock struct {
	TTL          time.Duration
	releaseDelay time.Duration
}

type NodeMeta struct {
	Unschedulable bool `json:"unschedulable"`
}

// DaemonSetLock holds all necessary information to do actions
// on the kured ds which holds lock info through annotations.
type DaemonSetLock struct {
	client     *kubernetes.Clientset
	nodeID     string
	namespace  string
	name       string
	annotation string
}

// DaemonSetSingleLock holds all necessary information to do actions
// on the kured ds which holds lock info through annotations.
type DaemonSetSingleLock struct {
	GenericLock
	DaemonSetLock
}

// DaemonSetMultiLock holds all necessary information to do actions
// on the kured ds which holds lock info through annotations, valid
// for multiple nodes
type DaemonSetMultiLock struct {
	GenericLock
	DaemonSetLock
	maxOwners int
}

// LockAnnotationValue contains the lock data,
// which allows persistence across reboots, particularily recording if the
// node was already unschedulable before kured reboot.
// To be modified when using another type of lock storage.
type LockAnnotationValue struct {
	NodeID   string        `json:"nodeID"`
	Metadata NodeMeta      `json:"metadata,omitempty"`
	Created  time.Time     `json:"created"`
	TTL      time.Duration `json:"TTL"`
}

type multiLockAnnotationValue struct {
	MaxOwners       int                   `json:"maxOwners"`
	LockAnnotations []LockAnnotationValue `json:"locks"`
}

// New creates a daemonsetLock object containing the necessary data for follow up k8s requests
func New(client *kubernetes.Clientset, nodeID, namespace, name, annotation string, TTL time.Duration, concurrency int, lockReleaseDelay time.Duration) Lock {
	if concurrency > 1 {
		return &DaemonSetMultiLock{
			GenericLock: GenericLock{
				TTL:          TTL,
				releaseDelay: lockReleaseDelay,
			},
			DaemonSetLock: DaemonSetLock{
				client:     client,
				nodeID:     nodeID,
				namespace:  namespace,
				name:       name,
				annotation: annotation,
			},
			maxOwners: concurrency,
		}
	} else {
		return &DaemonSetSingleLock{
			GenericLock: GenericLock{
				TTL:          TTL,
				releaseDelay: lockReleaseDelay,
			},
			DaemonSetLock: DaemonSetLock{
				client:     client,
				nodeID:     nodeID,
				namespace:  namespace,
				name:       name,
				annotation: annotation,
			},
		}
	}
}

// GetDaemonSet returns the named DaemonSet resource from the DaemonSetLock's configured client
func (dsl *DaemonSetLock) GetDaemonSet(sleep, timeout time.Duration) (*v1.DaemonSet, error) {
	var ds *v1.DaemonSet
	var lastError error
	err := wait.PollImmediate(sleep, timeout, func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if ds, lastError = dsl.client.AppsV1().DaemonSets(dsl.namespace).Get(ctx, dsl.name, metav1.GetOptions{}); lastError != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("Timed out trying to get daemonset %s in namespace %s: %v", dsl.name, dsl.namespace, lastError)
	}
	return ds, nil
}

// Acquire attempts to annotate the kured daemonset with lock info from instantiated DaemonSetLock using client-go
func (dsl *DaemonSetSingleLock) Acquire(nodeMetadata NodeMeta) (bool, error) {
	for {
		ds, err := dsl.GetDaemonSet(k8sAPICallRetrySleep, k8sAPICallRetryTimeout)
		if err != nil {
			return false, fmt.Errorf("timed out trying to get daemonset %s in namespace %s: %w", dsl.name, dsl.namespace, err)
		}

		valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
		if exists {
			value := LockAnnotationValue{}
			if err := json.Unmarshal([]byte(valueString), &value); err != nil {
				return false, err
			}

			if !ttlExpired(value.Created, value.TTL) {
				return value.NodeID == dsl.nodeID, nil
			}
		}

		if ds.ObjectMeta.Annotations == nil {
			ds.ObjectMeta.Annotations = make(map[string]string)
		}
		value := LockAnnotationValue{NodeID: dsl.nodeID, Metadata: nodeMetadata, Created: time.Now().UTC(), TTL: dsl.TTL}
		valueBytes, err := json.Marshal(&value)
		if err != nil {
			return false, err
		}
		ds.ObjectMeta.Annotations[dsl.annotation] = string(valueBytes)

		_, err = dsl.client.AppsV1().DaemonSets(dsl.namespace).Update(context.TODO(), ds, metav1.UpdateOptions{})
		if err != nil {
			if se, ok := err.(*errors.StatusError); ok && se.ErrStatus.Reason == metav1.StatusReasonConflict {
				// Something else updated the resource between us reading and writing - try again soon
				time.Sleep(time.Second)
				continue
			} else {
				return false, err
			}
		}
		return true, nil
	}
}

// GetNodeData returns the Node data fetched from lock. NodeMeta is only valid
// when holding is true.
func (dsl *DaemonSetSingleLock) GetNodeData() (holding bool, nodeMeta NodeMeta, err error) {

	ds, err := dsl.GetDaemonSet(k8sAPICallRetrySleep, k8sAPICallRetryTimeout)
	if err != nil {

		return false, nodeMeta, fmt.Errorf("timed out trying to get daemonset %s in namespace %s: %w", dsl.name, dsl.namespace, err)
	}

	valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
	if exists {
		value := LockAnnotationValue{}
		if err := json.Unmarshal([]byte(valueString), &value); err != nil {
			return false, nodeMeta, err
		}

		if !ttlExpired(value.Created, value.TTL) {
			return value.NodeID == dsl.nodeID, value.Metadata, nil
		}
	}

	return false, nodeMeta, nil
}

// Release attempts to remove the lock data from the kured ds annotations using client-go
func (dsl *DaemonSetSingleLock) Release() error {
	if dsl.releaseDelay > 0 {
		log.Infof("Waiting %v before releasing lock", dsl.releaseDelay)
		time.Sleep(dsl.releaseDelay)
	}
	for {
		ds, err := dsl.GetDaemonSet(k8sAPICallRetrySleep, k8sAPICallRetryTimeout)
		if err != nil {
			return fmt.Errorf("timed out trying to get daemonset %s in namespace %s: %w", dsl.name, dsl.namespace, err)
		}

		valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
		if !exists {
			return nil
		}
		value := LockAnnotationValue{}
		if err := json.Unmarshal([]byte(valueString), &value); err != nil {
			return err
		}

		if value.NodeID != dsl.nodeID {
			return nil
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

func (dsl *DaemonSetLock) canAcquireMultiple(annotation multiLockAnnotationValue, metadata NodeMeta, TTL time.Duration, maxOwners int) (bool, multiLockAnnotationValue) {
	newAnnotation := multiLockAnnotationValue{MaxOwners: maxOwners}
	freeSpace := false
	if annotation.LockAnnotations == nil || len(annotation.LockAnnotations) < maxOwners {
		freeSpace = true
		newAnnotation.LockAnnotations = annotation.LockAnnotations
	} else {
		for _, nodeLock := range annotation.LockAnnotations {
			if ttlExpired(nodeLock.Created, nodeLock.TTL) {
				freeSpace = true
				continue
			}
			newAnnotation.LockAnnotations = append(
				newAnnotation.LockAnnotations,
				nodeLock,
			)
		}
	}

	if freeSpace {
		newAnnotation.LockAnnotations = append(
			newAnnotation.LockAnnotations,
			LockAnnotationValue{
				NodeID:   dsl.nodeID,
				Metadata: metadata,
				Created:  time.Now().UTC(),
				TTL:      TTL,
			},
		)
		return true, newAnnotation
	}

	return false, multiLockAnnotationValue{}
}

// Acquire creates and annotates the daemonset with a multiple owner lock
func (dsl *DaemonSetMultiLock) Acquire(nodeMetaData NodeMeta) (bool, error) {
	for {
		ds, err := dsl.GetDaemonSet(k8sAPICallRetrySleep, k8sAPICallRetryTimeout)
		if err != nil {
			return false, fmt.Errorf("timed out trying to get daemonset %s in namespace %s: %w", dsl.name, dsl.namespace, err)
		}

		annotation := multiLockAnnotationValue{}
		valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
		if exists {
			if err := json.Unmarshal([]byte(valueString), &annotation); err != nil {
				return false, fmt.Errorf("error getting multi lock: %w", err)
			}
		}

		lockPossible, newAnnotation := dsl.canAcquireMultiple(annotation, nodeMetaData, dsl.TTL, dsl.maxOwners)
		if !lockPossible {
			return false, nil
		}

		if ds.ObjectMeta.Annotations == nil {
			ds.ObjectMeta.Annotations = make(map[string]string)
		}
		newAnnotationBytes, err := json.Marshal(&newAnnotation)
		if err != nil {
			return false, fmt.Errorf("error marshalling new annotation lock: %w", err)
		}
		ds.ObjectMeta.Annotations[dsl.annotation] = string(newAnnotationBytes)

		_, err = dsl.client.AppsV1().DaemonSets(dsl.namespace).Update(context.Background(), ds, metav1.UpdateOptions{})
		if err != nil {
			if se, ok := err.(*errors.StatusError); ok && se.ErrStatus.Reason == metav1.StatusReasonConflict {
				time.Sleep(time.Second)
				continue
			} else {
				return false, fmt.Errorf("error updating daemonset with multi lock: %w", err)
			}
		}
		return true, nil
	}
}

// GetNodeData returns the Node data fetched from lock. NodeMeta is only valid
// when holding is true.
func (dsl *DaemonSetMultiLock) GetNodeData() (holding bool, nodeMeta NodeMeta, err error) {
	ds, err := dsl.GetDaemonSet(k8sAPICallRetrySleep, k8sAPICallRetryTimeout)
	if err != nil {
		return false, nodeMeta, err
	}

	valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
	if exists {
		value := multiLockAnnotationValue{}
		if err := json.Unmarshal([]byte(valueString), &value); err != nil {
			return false, nodeMeta, err
		}

		for _, nodeLock := range value.LockAnnotations {
			if nodeLock.NodeID == dsl.nodeID && !ttlExpired(nodeLock.Created, nodeLock.TTL) {
				return true, nodeLock.Metadata, nil
			}
		}
	}

	return false, nodeMeta, nil
}

// Release attempts to remove the lock data for a single node from the multi node annotation
func (dsl *DaemonSetMultiLock) Release() error {
	if dsl.releaseDelay > 0 {
		log.Infof("Waiting %v before releasing lock", dsl.releaseDelay)
		time.Sleep(dsl.releaseDelay)
	}
	for {
		ds, err := dsl.GetDaemonSet(k8sAPICallRetrySleep, k8sAPICallRetryTimeout)
		if err != nil {
			return fmt.Errorf("timed out trying to get daemonset %s in namespace %s: %w", dsl.name, dsl.namespace, err)
		}

		valueString, exists := ds.ObjectMeta.Annotations[dsl.annotation]
		modified := false
		value := multiLockAnnotationValue{}
		if exists {
			if err := json.Unmarshal([]byte(valueString), &value); err != nil {
				return err
			}

			for idx, nodeLock := range value.LockAnnotations {
				if nodeLock.NodeID == dsl.nodeID {
					value.LockAnnotations = append(value.LockAnnotations[:idx], value.LockAnnotations[idx+1:]...)
					modified = true
					break
				}
			}
		}

		if !exists || !modified {
			return nil
		}

		newAnnotationBytes, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("error marshalling new annotation on release: %v", err)
		}
		ds.ObjectMeta.Annotations[dsl.annotation] = string(newAnnotationBytes)

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
