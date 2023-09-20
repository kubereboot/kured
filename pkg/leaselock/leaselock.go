package leaselock

import (
	"context"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

var leaderMutex sync.Mutex
var isLeading bool
var leaderIndex int

// LeaseLock holds all necessary information to do actions
// on the kured leases which holds lock info through annotations.
type LeaseLock struct {
	client      *kubernetes.Clientset
	nodeID      string
	namespace   string
	name        string
	concurrency int
}

// New creates a LeaseLock object containing the necessary data for follow up k8s requests
func New(client *kubernetes.Clientset, nodeID, namespace, name string, concurrency int) *LeaseLock {
	return &LeaseLock{client, nodeID, namespace, name, concurrency}
}

// Acquire attempts to annotate the kured Leases with lock info from instantiated LeaseLock using client-go
func (l *LeaseLock) Acquire(TTL time.Duration, restartFunc func()) error {

	for i := 0; i < l.concurrency; i++ {
		ctx, _ := context.WithCancel(context.Background())

		lock := &resourcelock.LeaseLock{
			// configure LeaseLock
			Client: l.client.CoordinationV1(),
			LeaseMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%d", l.name, i),
				Namespace: l.namespace,
			},
			LockConfig: resourcelock.ResourceLockConfig{Identity: l.nodeID},
		}

		go leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
			ReleaseOnCancel: true,
			RenewDeadline:   15 * time.Second,
			RetryPeriod:     5 * time.Second,
			LeaseDuration:   TTL,
			Lock:            lock,
			Callbacks: leaderelection.LeaderCallbacks{
				OnStartedLeading: func(ctx context.Context) {
					leaderMutex.Lock()
					defer leaderMutex.Unlock()
					if isLeading {
						ctx.Done()
					}
					log.Infof("Acquired lock")
					isLeading = true
					leaderIndex = i
					restartFunc()
				},
			},
		})
	}
	return nil
}

// Test attempts to check the kured Lease lock status (existence, expiry) from instantiated Lease using client-go
func (l *LeaseLock) Test() (bool, error) {
	leaseClient := l.client.CoordinationV1()
	for i := 0; i < l.concurrency; i++ {
		lease, err := leaseClient.Leases(l.namespace).Get(context.TODO(), fmt.Sprintf("%s-%d", l.name, i), metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		if lease.Spec.HolderIdentity == &l.nodeID {
			leaderMutex.Lock()
			defer leaderMutex.Unlock()
			isLeading = true
			leaderIndex = i
			return true, nil
		}
	}
	return false, nil
}

// Release attempts to remove the lock data from the kured Lease annotations using client-go
func (l *LeaseLock) Release() error {
	leaseClient := l.client.CoordinationV1()
	if isLeading {
		lease, err := leaseClient.Leases(l.namespace).Get(context.TODO(), fmt.Sprintf("%s-%d", l.name, leaderIndex), metav1.GetOptions{})
		if err != nil {
			return err
		}
		lease.Spec.HolderIdentity = nil
		_, err = leaseClient.Leases(l.namespace).Update(context.TODO(), lease, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
