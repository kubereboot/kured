package main

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if os.Getenv("KURED_TEST_RUN_MAIN") == "1" && len(os.Args) > 1 && os.Args[1] == "--" {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func kuredTestEnv() []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "KURED_") ||
			strings.HasPrefix(entry, "KUBERNETES_SERVICE_HOST=") ||
			strings.HasPrefix(entry, "KUBERNETES_SERVICE_PORT=") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "KURED_TEST_RUN_MAIN=1")
}

func runKured(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"--"}, args...)...)
	cmd.Env = kuredTestEnv()
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("running kured: %v\n%s", err, out)
	return string(out), -1
}

func TestCommandTimeoutFlagHelp(t *testing.T) {
	out, code := runKured(t, "--help")
	if code != 0 {
		t.Fatalf("help exit %d: %s", code, out)
	}
	for _, fragment := range []string{
		"--sentinel-command-timeout",
		"--reboot-command-timeout",
		"positive maximum duration",
		"default 60s",
		"inherited stdout/stderr",
		"1s drain grace",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("help missing %q\n%s", fragment, out)
		}
	}
	if got := strings.Count(out, "(default 1m0s)"); got != 2 {
		t.Errorf("(default 1m0s) count = %d, want 2\n%s", got, out)
	}
}

func TestCommandTimeoutFlagsRejectNonPositiveBeforeClusterConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "sentinel zero", args: []string{"--node-id=node", "--sentinel-command-timeout=0s"}, want: "sentinel-command-timeout"},
		{name: "sentinel negative", args: []string{"--node-id=node", "--sentinel-command-timeout=-1s"}, want: "sentinel-command-timeout"},
		{name: "reboot zero", args: []string{"--node-id=node", "--reboot-command-timeout=0s"}, want: "reboot-command-timeout"},
		{name: "reboot negative", args: []string{"--node-id=node", "--reboot-command-timeout=-1s"}, want: "reboot-command-timeout"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, code := runKured(t, tt.args...)
			if code != 1 {
				t.Fatalf("exit %d, want 1: %s", code, out)
			}
			if strings.Contains(out, "unable to load in-cluster configuration") {
				t.Fatalf("validation reached Kubernetes configuration: %s", out)
			}
			if !strings.Contains(out, tt.want) || !strings.Contains(out, "must be positive") {
				t.Fatalf("output = %s", out)
			}
		})
	}
}

func TestPositiveCommandTimeoutsReachClusterConfig(t *testing.T) {
	tests := [][]string{
		{"--node-id=node"},
		{"--node-id=node", "--sentinel-command-timeout=30s", "--reboot-command-timeout=2m"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, code := runKured(t, args...)
			if code == 0 {
				t.Fatalf("expected failure outside a cluster: %s", out)
			}
			if strings.Contains(out, "must be positive") {
				t.Fatalf("positive timeouts were rejected: %s", out)
			}
			if !strings.Contains(out, "unable to load in-cluster configuration") {
				t.Fatalf("expected cluster configuration access, got: %s", out)
			}
		})
	}
}

func TestValidateNotificationURL(t *testing.T) {

	tests := []struct {
		name         string
		slackHookURL string
		notifyURL    string
		expected     string
	}{
		{"slackHookURL only works fine", "https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"slackHookURL and notify URL together only keeps notifyURL", "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\"", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"},
		{"slackHookURL removes extraneous double quotes", "\"https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET\"", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"slackHookURL removes extraneous single quotes", "'https://hooks.slack.com/services/BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET'", "", "slack://BLABLABA12345/IAM931A0VERY/COMPLICATED711854TOKEN1SET"},
		{"notifyURL removes extraneous double quotes", "", "\"teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com\"", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"},
		{"notifyURL removes extraneous single quotes", "", "'teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com'", "teams://79b4XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX@acd8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX/204cXXXXXXXXXXXXXXXXXXXXXXXXXXXX/a1f8XXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX?host=XXXX.webhook.office.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateNotificationURL(tt.notifyURL, tt.slackHookURL); !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("validateNotificationURL() = %v, expected %v", got, tt.expected)
			}
		})
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
