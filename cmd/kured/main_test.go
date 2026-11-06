package main

import (
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"
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
	return runKuredWithEnv(t, nil, args...)
}

func runKuredWithEnv(t *testing.T, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"--"}, args...)...)
	cmd.Env = append(kuredTestEnv(), env...)
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
		"before it is killed, must be positive",
		"0 (the default) means no deadline",
		"waits at most 1s more",
		"still holding its stdout or stderr",
	} {
		if !strings.Contains(out, fragment) {
			t.Errorf("help missing %q\n%s", fragment, out)
		}
	}
	if got := strings.Count(out, "(default 1m0s)"); got != 1 {
		t.Errorf("(default 1m0s) count = %d, want 1 for the sentinel timeout only\n%s", got, out)
	}
}

func TestCommandTimeoutFlagsRejectInvalidBeforeClusterConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "sentinel zero", args: []string{"--node-id=node", "--sentinel-command-timeout=0s"}, want: "sentinel-command-timeout must be positive"},
		{name: "sentinel negative", args: []string{"--node-id=node", "--sentinel-command-timeout=-1s"}, want: "sentinel-command-timeout must be positive"},
		{name: "reboot negative", args: []string{"--node-id=node", "--reboot-command-timeout=-1s"}, want: "reboot-command-timeout must not be negative"},
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
			if !strings.Contains(out, tt.want) {
				t.Fatalf("output = %s", out)
			}
		})
	}
}

func TestValidCommandTimeoutsReachClusterConfig(t *testing.T) {
	tests := [][]string{
		{"--node-id=node"},
		{"--node-id=node", "--sentinel-command-timeout=30s", "--reboot-command-timeout=2m"},
		{"--node-id=node", "--reboot-command-timeout=0s"},
		{"--node-id=node", "--reboot-sentinel-command=true", "--period=2m", "--sentinel-command-timeout=1m"},
		// No sentinel command, so the sentinel timeout is unused.
		{"--node-id=node", "--period=30s", "--sentinel-command-timeout=1m"},
		// The reboot command runs once and never meets the next period tick.
		{"--node-id=node", "--period=30s", "--reboot-command-timeout=1m"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, code := runKured(t, args...)
			if code == 0 {
				t.Fatalf("expected failure outside a cluster: %s", out)
			}
			if strings.Contains(out, "must be positive") || strings.Contains(out, "must not be negative") || strings.Contains(out, "must be shorter") {
				t.Fatalf("valid timeouts were rejected: %s", out)
			}
			if !strings.Contains(out, "unable to load in-cluster configuration") {
				t.Fatalf("expected cluster configuration access, got: %s", out)
			}
		})
	}
}

func TestValidateSentinelTimeout(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		timeout     time.Duration
		period      time.Duration
		explicit    bool
		wantWarning bool
		wantErr     bool
	}{
		{name: "shorter than period", command: "true", timeout: time.Minute, period: time.Hour, explicit: true},
		{name: "explicit equal to period", command: "true", timeout: time.Hour, period: time.Hour, explicit: true, wantErr: true},
		{name: "explicit longer than period", command: "true", timeout: 2 * time.Hour, period: time.Hour, explicit: true, wantErr: true},
		{name: "default equal to period", command: "true", timeout: time.Minute, period: time.Minute, wantWarning: true},
		{name: "default longer than period", command: "true", timeout: time.Minute, period: 20 * time.Second, wantWarning: true},
		{name: "sentinel file ignores explicit timeout", timeout: time.Hour, period: time.Minute, explicit: true},
		{name: "sentinel file ignores default timeout", timeout: time.Minute, period: 20 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warning, err := validateSentinelTimeout(tt.command, tt.timeout, tt.period, tt.explicit)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if (warning != "") != tt.wantWarning {
				t.Fatalf("warning = %q, wantWarning %v", warning, tt.wantWarning)
			}
		})
	}
}

func TestSentinelTimeoutMustFitPeriod(t *testing.T) {
	const want = "sentinel-command-timeout (1m0s) must be shorter than period (30s)"
	args := []string{"--node-id=node", "--reboot-sentinel-command=true", "--period=30s"}

	t.Run("flag", func(t *testing.T) {
		out, code := runKured(t, append(args, "--sentinel-command-timeout=1m")...)
		if code != 1 || !strings.Contains(out, want) {
			t.Fatalf("exit %d, output = %s", code, out)
		}
		if strings.Contains(out, "unable to load in-cluster configuration") {
			t.Fatalf("validation reached Kubernetes configuration: %s", out)
		}
	})

	t.Run("environment", func(t *testing.T) {
		out, code := runKuredWithEnv(t, []string{"KURED_SENTINEL_COMMAND_TIMEOUT=1m"}, args...)
		if code != 1 || !strings.Contains(out, want) {
			t.Fatalf("exit %d, output = %s", code, out)
		}
	})
}

func TestDefaultSentinelTimeoutOverPeriodOnlyWarns(t *testing.T) {
	out, code := runKured(t, "--node-id=node", "--reboot-sentinel-command=true", "--period=30s")
	if code == 0 {
		t.Fatalf("expected failure outside a cluster: %s", out)
	}
	if !strings.Contains(out, "is not shorter than period (30s)") {
		t.Fatalf("missing warning: %s", out)
	}
	if !strings.Contains(out, "unable to load in-cluster configuration") {
		t.Fatalf("expected cluster configuration access, got: %s", out)
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
