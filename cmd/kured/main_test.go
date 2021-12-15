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

func Test_getSentinelCommand_Linux(t *testing.T) {
	type args struct {
		pid                   int
		rebootSentinelFile    string
		rebootSentinelCommand string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Ensure a sentinelFile generates a shell 'test' command with the right file",
			args: args{
				pid:                   1,
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "",
			},
			want: []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "test", "-f", "/test1"},
		},
		{
			name: "Ensure a sentinelCommand has priority over a sentinelFile if both are provided (because sentinelFile is always provided)",
			args: args{
				pid:                   1,
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "/sbin/reboot-required -r",
			},
			want: []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "/sbin/reboot-required", "-r"},
		},
		{
			name: "Ensure specified PID is used in commands",
			args: args{
				pid:                   5,
				rebootSentinelFile:    "/foo",
				rebootSentinelCommand: "",
			},
			want: []string{"/usr/bin/nsenter", "-m/proc/5/ns/mnt", "--", "test", "-f", "/foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linuxHelper := linuxHelper{pid: tt.args.pid}
			if got := linuxHelper.getSentinelCommand(tt.args.rebootSentinelFile, tt.args.rebootSentinelCommand); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSentinelCommand() for Linux = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_GetSentinelCommand_Windows(t *testing.T) {
	type args struct {
		rebootSentinelFile    string
		rebootSentinelCommand string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Ensure Test-PendingReboot.ps1 is used regardless of sentinelFile",
			args: args{
				rebootSentinelFile:    "/foo",
				rebootSentinelCommand: "",
			},
			want: []string{"powershell.exe", "./Test-PendingReboot.ps1"},
		},
		{
			name: "Ensure Test-PendingReboot.ps1 is used regardless of sentinelCommand",
			args: args{
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "/sbin/reboot-required -r",
			},
			want: []string{"powershell.exe", "./Test-PendingReboot.ps1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windowsHelper := windowsHelper{}
			if got := windowsHelper.getSentinelCommand(tt.args.rebootSentinelFile, tt.args.rebootSentinelCommand); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSentinelCommand() for Windows = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getRebootCommand_Linux(t *testing.T) {
	type args struct {
		rebootCommand string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Ensure a reboot command is properly parsed",
			args: args{
				rebootCommand: "/sbin/systemctl reboot",
			},
			want: []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "/sbin/systemctl", "reboot"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			linuxHelper := linuxHelper{pid: 1}
			if got := linuxHelper.getRebootCommand(tt.args.rebootCommand); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRebootCommand() for Linux = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getRebootCommand_Windows(t *testing.T) {
	type args struct {
		rebootCommand string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Ensure a correct reboot command is used for Windows",
			args: args{
				rebootCommand: "/sbin/systemctl reboot",
			},
			want: []string{"shutdown.exe", "/f", "/r", "/t", "5", "/c", "kured"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			windowsHelper := windowsHelper{}
			if got := windowsHelper.getRebootCommand(tt.args.rebootCommand); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRebootCommand() for Windows = %v, want %v", got, tt.want)
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
