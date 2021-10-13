package main

import (
	"reflect"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/weaveworks/kured/pkg/alerts"
	assert "gotest.tools/v3/assert"

	papi "github.com/prometheus/client_golang/api"
)

type BlockingChecker struct {
	blocking bool
}

func (fbc BlockingChecker) isBlocked() bool {
	return fbc.blocking
}

var _ RebootBlocker = BlockingChecker{}       // Verify that Type implements Interface.
var _ RebootBlocker = (*BlockingChecker)(nil) // Verify that *Type implements Interface.

func Test_flagCheck(t *testing.T) {
	var cmd *cobra.Command
	var args []string
	slackHookURL = "https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	flagCheck(cmd, args)
	if notifyURL != "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET" {
		t.Errorf("Slack URL Parsing is wrong: expecting %s  but got %s\n", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET", notifyURL)
	}
}
func Test_rebootBlocked(t *testing.T) {
	noCheckers := []RebootBlocker{}
	nonblockingChecker := BlockingChecker{blocking: false}
	blockingChecker := BlockingChecker{blocking: true}

	// Instantiate a prometheusClient with a broken_url
	promClient, err := alerts.NewPromClient(papi.Config{Address: "broken_url"})
	if err != nil {
		log.Fatal("Can't create prometheusClient: ", err)
	}
	brokenPrometheusClient := PrometheusBlockingChecker{promClient: promClient, filter: nil, firingOnly: false}

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

func Test_buildHostCommand(t *testing.T) {
	type args struct {
		pid     int
		command []string
	}
	tests := []struct {
		name string
		args args
		GOOS string
		want []string
	}{
		{
			name: "Ensure command will run with nsenter on Linux",
			args: args{pid: 1, command: []string{"ls", "-Fal"}},
			GOOS: "linux",
			want: []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "ls", "-Fal"},
		},
		{
			name: "Ensure command runs as specified on Windows",
			args: args{pid: 1, command: []string{"powershell", "ls"}},
			GOOS: "windows",
			want: []string{"powershell", "ls"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildHostCommand(tt.args.pid, tt.args.command, tt.GOOS); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildHostCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_buildSentinelCommand(t *testing.T) {
	type args struct {
		rebootSentinelFile    string
		rebootSentinelCommand string
	}
	tests := []struct {
		name string
		args args
		GOOS string
		want []string
	}{
		{
			name: "Ensure a sentinelFile generates a shell 'test' command with the right file",
			args: args{
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "",
			},
			GOOS: "linux",
			want: []string{"test", "-f", "/test1"},
		},
		{
			name: "Ensure a sentinelCommand has priority over a sentinelFile if both are provided (because sentinelFile is always provided)",
			args: args{
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "/sbin/reboot-required -r",
			},
			GOOS: "linux",
			want: []string{"/sbin/reboot-required", "-r"},
		},
		{
			name: "Ensure a sentinelFile on generates a powershell command with the right file on Windows",
			args: args{
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "",
			},
			GOOS: "windows",
			want: []string{"powershell.exe", "/c", "if", "(Test-Path", "/test1)", "{exit", "0}", "else", "{exit", "1}"},
		},
		{
			name: "Ensure a sentinelCommand has priority over sentinelFile if both are provided on Windows",
			args: args{
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "powershell.exe ./Test-RebootRequired",
			},
			GOOS: "windows",
			want: []string{"powershell.exe", "./Test-RebootRequired"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildSentinelCommand(tt.args.rebootSentinelFile, tt.args.rebootSentinelCommand, tt.GOOS); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildSentinelCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseRebootCommand(t *testing.T) {
	type args struct {
		rebootCommand string
	}
	tests := []struct {
		name string
		args args
		GOOS string
		want []string
	}{
		{
			name: "Ensure a reboot command is properly parsed",
			args: args{
				rebootCommand: "/sbin/systemctl reboot",
			},
			want: []string{"/sbin/systemctl", "reboot"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRebootCommand(tt.args.rebootCommand); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRebootCommand() = %v, want %v", got, tt.want)
			}
		})
	}
}

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
			if got := rebootRequired(tt.args.sentinelCommand); got != tt.want {
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
		rebootRequired(c.param)
		assert.Equal(t, c.expectFatal, fatal)
	}

}
