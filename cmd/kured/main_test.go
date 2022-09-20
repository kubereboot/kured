package main

import (
	"reflect"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/kubereboot/kured/pkg/alerts"
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
	expected := "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("Slack URL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}

	// validate that surrounding quotes are stripped
	slackHookURL = "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\""
	expected = "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("Slack URL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}
	slackHookURL = "'https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET'"
	expected = "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("Slack URL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}
	slackHookURL = ""
	notifyURL = "\"teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com\""
	expected = "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("notifyURL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}
	notifyURL = "'teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com'"
	expected = "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"
	flagCheck(cmd, args)
	if notifyURL != expected {
		t.Errorf("notifyURL Parsing is wrong: expecting %s  but got %s\n", expected, notifyURL)
	}
}

func Test_stripQuotes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string with no surrounding quotes is unchanged",
			input:    "Hello, world!",
			expected: "Hello, world!",
		},
		{
			name:     "string with surrounding double quotes should strip quotes",
			input:    "\"Hello, world!\"",
			expected: "Hello, world!",
		},
		{
			name:     "string with surrounding single quotes should strip quotes",
			input:    "'Hello, world!'",
			expected: "Hello, world!",
		},
		{
			name:     "string with unbalanced surrounding quotes is unchanged",
			input:    "'Hello, world!\"",
			expected: "'Hello, world!\"",
		},
		{
			name:     "string with length of one is unchanged",
			input:    "'",
			expected: "'",
		},
		{
			name:     "string with length of zero is unchanged",
			input:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripQuotes(tt.input); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("stripQuotes() = %v, expected %v", got, tt.expected)
			}
		})
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
			if got := buildHostCommand(tt.args.pid, tt.args.command); !reflect.DeepEqual(got, tt.want) {
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
		want []string
	}{
		{
			name: "Ensure a sentinelFile generates a shell 'test' command with the right file",
			args: args{
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "",
			},
			want: []string{"test", "-f", "/test1"},
		},
		{
			name: "Ensure a sentinelCommand has priority over a sentinelFile if both are provided (because sentinelFile is always provided)",
			args: args{
				rebootSentinelFile:    "/test1",
				rebootSentinelCommand: "/sbin/reboot-required -r",
			},
			want: []string{"/sbin/reboot-required", "-r"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildSentinelCommand(tt.args.rebootSentinelFile, tt.args.rebootSentinelCommand); !reflect.DeepEqual(got, tt.want) {
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
