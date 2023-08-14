package daemonsetlock

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestTtlExpired(t *testing.T) {
	d := time.Date(2020, 05, 05, 14, 15, 0, 0, time.UTC)
	second, _ := time.ParseDuration("1s")
	zero, _ := time.ParseDuration("0m")

	tests := []struct {
		created time.Time
		ttl     time.Duration
		result  bool
	}{
		{d, second, true},
		{time.Now(), second, false},
		{d, zero, false},
	}

	for i, tst := range tests {
		if ttlExpired(tst.created, tst.ttl) != tst.result {
			t.Errorf("Test %d failed, expected %v but got %v", i, tst.result, !tst.result)
		}
	}
}

func multiLockAnnotationsAreEqualByNodes(src, dst multiLockAnnotationValue) bool {
	srcNodes := []string{}
	for _, srcLock := range src.LockAnnotations {
		srcNodes = append(srcNodes, srcLock.NodeID)
	}
	sort.Strings(srcNodes)

	dstNodes := []string{}
	for _, dstLock := range dst.LockAnnotations {
		dstNodes = append(dstNodes, dstLock.NodeID)
	}
	sort.Strings(dstNodes)

	return reflect.DeepEqual(srcNodes, dstNodes)
}

func TestCanAcquireMultiple(t *testing.T) {
	node1Name := "n1"
	node2Name := "n2"
	node3Name := "n3"
	testCases := []struct {
		name          string
		daemonSetLock DaemonSetLock
		maxOwners     int
		current       multiLockAnnotationValue
		desired       multiLockAnnotationValue
		lockPossible  bool
	}{
		{
			name: "empty_lock",
			daemonSetLock: DaemonSetLock{
				nodeID: node1Name,
			},
			maxOwners: 2,
			current:   multiLockAnnotationValue{},
			desired: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{NodeID: node1Name},
				},
			},
			lockPossible: true,
		},
		{
			name: "partial_lock",
			daemonSetLock: DaemonSetLock{
				nodeID: node1Name,
			},
			maxOwners: 2,
			current: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{NodeID: node2Name},
				},
			},
			desired: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{NodeID: node1Name},
					{NodeID: node2Name},
				},
			},
			lockPossible: true,
		},
		{
			name: "full_lock",
			daemonSetLock: DaemonSetLock{
				nodeID: node1Name,
			},
			maxOwners: 2,
			current: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{
						NodeID:  node2Name,
						Created: time.Now().UTC().Add(-1 * time.Minute),
						TTL:     time.Hour,
					},
					{
						NodeID:  node3Name,
						Created: time.Now().UTC().Add(-1 * time.Minute),
						TTL:     time.Hour,
					},
				},
			},
			desired: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{NodeID: node2Name},
					{NodeID: node3Name},
				},
			},
			lockPossible: false,
		},
		{
			name: "full_with_one_expired_lock",
			daemonSetLock: DaemonSetLock{
				nodeID: node1Name,
			},
			maxOwners: 2,
			current: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{
						NodeID:  node2Name,
						Created: time.Now().UTC().Add(-1 * time.Hour),
						TTL:     time.Minute,
					},
					{
						NodeID:  node3Name,
						Created: time.Now().UTC().Add(-1 * time.Minute),
						TTL:     time.Hour,
					},
				},
			},
			desired: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{NodeID: node1Name},
					{NodeID: node3Name},
				},
			},
			lockPossible: true,
		},
		{
			name: "full_with_all_expired_locks",
			daemonSetLock: DaemonSetLock{
				nodeID: node1Name,
			},
			maxOwners: 2,
			current: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{
						NodeID:  node2Name,
						Created: time.Now().UTC().Add(-1 * time.Hour),
						TTL:     time.Minute,
					},
					{
						NodeID:  node3Name,
						Created: time.Now().UTC().Add(-1 * time.Hour),
						TTL:     time.Minute,
					},
				},
			},
			desired: multiLockAnnotationValue{
				MaxOwners: 2,
				LockAnnotations: []lockAnnotationValue{
					{NodeID: node1Name},
				},
			},
			lockPossible: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			lockPossible, actual := testCase.daemonSetLock.canAcquireMultiple(testCase.current, struct{}{}, time.Minute, testCase.maxOwners)
			if lockPossible != testCase.lockPossible {
				t.Fatalf(
					"unexpected result for lock possible (got %t expected %t new annotation %v",
					lockPossible,
					testCase.lockPossible,
					actual,
				)
			}

			if lockPossible && (!multiLockAnnotationsAreEqualByNodes(actual, testCase.desired) || testCase.desired.MaxOwners != actual.MaxOwners) {
				t.Fatalf(
					"expected lock %v but got %v",
					testCase.desired,
					actual,
				)
			}
		})
	}
}
