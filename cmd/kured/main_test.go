package main

import "testing"

type BlockingChecker struct {
	blocking bool
}

func (fbc BlockingChecker) isBlocked() bool {
	return fbc.blocking
}

var _ RebootBlocker = BlockingChecker{}       // Verify that Type implements Interface.
var _ RebootBlocker = (*BlockingChecker)(nil) // Verify that *Type implements Interface.

func Test_rebootBlocked(t *testing.T) {
	noCheckers := []RebootBlocker{}
	nonblockingChecker := BlockingChecker{blocking: false}
	blockingChecker := BlockingChecker{blocking: true}
	brokenPrometheusClient := PrometheusBlockingChecker{promURL: "", filter: nil}

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
			if got := rebootBlocked(tt.args.blockers...); got != tt.want {
				t.Errorf("rebootBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}
