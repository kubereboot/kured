package checkers

import (
	log "github.com/sirupsen/logrus"
	"reflect"
	"testing"
)

func Test_nsEntering(t *testing.T) {
	type args struct {
		pid        int
		command    string
		privileged bool
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Ensure command will run with nsenter",
			args: args{pid: 1, command: "ls -Fal", privileged: true},
			want: []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "ls", "-Fal"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cc, _ := NewCommandChecker(tt.args.command, tt.args.pid, tt.args.privileged)
			if !reflect.DeepEqual(cc.CheckCommand, tt.want) {
				t.Errorf("command parsed as %v, want %v", cc.CheckCommand, tt.want)
			}
		})
	}
}

func Test_rebootRequired(t *testing.T) {
	type args struct {
		sentinelCommand []string
	}
	tests := []struct {
		name   string
		args   args
		want   bool
		fatals bool
	}{
		{
			name: "Ensure rc = 0 means reboot required",
			args: args{
				sentinelCommand: []string{"true"},
			},
			want:   true,
			fatals: false,
		},
		{
			name: "Ensure rc != 0 means reboot NOT required",
			args: args{
				sentinelCommand: []string{"false"},
			},
			want:   false,
			fatals: false,
		},
		{
			name: "Ensure a wrong command fatals",
			args: args{
				sentinelCommand: []string{"./babar"},
			},
			want:   true,
			fatals: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() { log.StandardLogger().ExitFunc = nil }()
			fatal := false
			log.StandardLogger().ExitFunc = func(int) { fatal = true }

			a := CommandChecker{CheckCommand: tt.args.sentinelCommand, NamespacePid: 1, Privileged: false}

			if got := a.RebootRequired(); got != tt.want {
				t.Errorf("rebootRequired() = %v, want %v", got, tt.want)
			}
			if tt.fatals != fatal {
				t.Errorf("fatal flag is %v, want fatal %v", fatal, tt.fatals)
			}
		})
	}
}
