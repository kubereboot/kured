package checkers

import (
	log "github.com/sirupsen/logrus"
	assert "gotest.tools/v3/assert"
	"testing"
)

func Test_rebootRequired(t *testing.T) {
	type args struct {
		sentinelCommand []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Ensure rc = 0 means reboot required",
			args: args{
				sentinelCommand: []string{"true"},
			},
			want: true,
		},
		{
			name: "Ensure rc != 0 means reboot NOT required",
			args: args{
				sentinelCommand: []string{"false"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := CommandChecker{CheckCommand: tt.args.sentinelCommand, NamespacePid: 1, Privileged: false}
			if got := a.RebootRequired(); got != tt.want {
				t.Errorf("rebootRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_rebootRequired_fatals(t *testing.T) {
	cases := []struct {
		param       []string
		expectFatal bool
	}{
		{
			param:       []string{"true"},
			expectFatal: false,
		},
		{
			param:       []string{"./babar"},
			expectFatal: true,
		},
	}

	defer func() { log.StandardLogger().ExitFunc = nil }()
	var fatal bool
	log.StandardLogger().ExitFunc = func(int) { fatal = true }

	for _, c := range cases {
		fatal = false
		a := CommandChecker{CheckCommand: c.param, NamespacePid: 1, Privileged: false}
		a.RebootRequired()
		assert.Equal(t, c.expectFatal, fatal)
	}

}
