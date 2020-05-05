package daemonsetlock

import (
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
