package util

import (
	"reflect"
	"testing"
)

func Test_buildHostCommand(t *testing.T) {
	type args struct {
		pid     int
		command []string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Ensure command will run with nsenter",
			args: args{pid: 1, command: []string{"ls", "-Fal"}},
			want: []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "ls", "-Fal"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PrivilegedHostCommand(tt.args.pid, tt.args.command); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildHostCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}
