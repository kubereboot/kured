package blockers

import (
	papi "github.com/prometheus/client_golang/api"
	"testing"
)

type BlockingChecker struct {
	blocking bool
}

func (fbc BlockingChecker) IsBlocked() bool {
	return fbc.blocking
}

func Test_rebootBlocked(t *testing.T) {
	noCheckers := []RebootBlocker{}
	nonblockingChecker := BlockingChecker{blocking: false}
	blockingChecker := BlockingChecker{blocking: true}

	// Instantiate a prometheusClient with a broken_url
	brokenPrometheusClient := NewPrometheusBlockingChecker(papi.Config{Address: "broken_url"}, nil, false, false)

	type args struct {
		blockers []RebootBlocker
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Do not block on no blocker defined",
			args: args{blockers: noCheckers},
			want: false,
		},
		{
			name: "Ensure a blocker blocks",
			args: args{blockers: []RebootBlocker{blockingChecker}},
			want: true,
		},
		{
			name: "Ensure a non-blocker doesn't block",
			args: args{blockers: []RebootBlocker{nonblockingChecker}},
			want: false,
		},
		{
			name: "Ensure one blocker is enough to block",
			args: args{blockers: []RebootBlocker{nonblockingChecker, blockingChecker}},
			want: true,
		},
		{
			name: "Do block on error contacting prometheus API",
			args: args{blockers: []RebootBlocker{brokenPrometheusClient}},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RebootBlocked(tt.args.blockers...); got != tt.want {
				t.Errorf("rebootBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}
