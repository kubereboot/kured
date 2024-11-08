package reboot

import (
	"reflect"
	"testing"
)

func TestNewCommandRebooter(t *testing.T) {
	type args struct {
		rebootCommand string
	}
	tests := []struct {
		name    string
		args    args
		want    *CommandRebooter
		wantErr bool
	}{
		{
			name:    "Ensure command is nsenter wrapped",
			args:    args{"ls -Fal"},
			want:    &CommandRebooter{RebootCommand: []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "ls", "-Fal"}},
			wantErr: false,
		},
		{
			name:    "Ensure empty command is erroring",
			args:    args{""},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewCommandRebooter(tt.args.rebootCommand, 0, true, 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCommandRebooter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewCommandRebooter() got = %v, want %v", got, tt.want)
			}
		})
	}
}
