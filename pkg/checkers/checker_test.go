package checkers

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
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

func Test_NewFileRebootChecker(t *testing.T) {
	if _, err := NewFileRebootChecker(""); err == nil {
		t.Error("expected an error for an empty file path, got nil")
	}
	if _, err := NewFileRebootChecker("   "); err == nil {
		t.Error("expected an error for a blank file path, got nil")
	}
	c, err := NewFileRebootChecker("/var/run/reboot-required")
	if err != nil {
		t.Fatalf("unexpected error for a valid file path: %v", err)
	}
	if c.FilePath != "/var/run/reboot-required" {
		t.Errorf("FilePath = %q, want %q", c.FilePath, "/var/run/reboot-required")
	}
	c, err = NewFileRebootChecker("  /var/run/reboot-required  ")
	if err != nil {
		t.Fatalf("unexpected error for a padded file path: %v", err)
	}
	if c.FilePath != "/var/run/reboot-required" {
		t.Errorf("FilePath = %q, want trimmed %q", c.FilePath, "/var/run/reboot-required")
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

const (
	stdoutMarker     = "captured-stdout"
	stderrMarker     = "captured-stderr"
	freshStdout      = "fresh-stdout"
	freshStderr      = "fresh-stderr"
	blockerSeconds   = 30
	scheduleSlack    = 8 * time.Second
	shortDeadline    = 400 * time.Millisecond
	descendantLimit  = 3 * time.Second
	drainGraceBudget = 15 * time.Second
)

func shell(script string) []string {
	return []string{"/bin/sh", "-c", script}
}

func markedExit(code int) string {
	return fmt.Sprintf("echo %s; echo %s >&2; exit %d", stdoutMarker, stderrMarker, code)
}

func sleepingCommand() string {
	return fmt.Sprintf("echo %s; echo %s >&2; exec sleep %d", stdoutMarker, stderrMarker, blockerSeconds)
}

func descendantScript(pidFile string, parentTail string) string {
	return fmt.Sprintf("echo %s; echo %s >&2; sleep %d >/dev/stdout 2>/dev/stderr & echo $! > %q; %s",
		stdoutMarker, stderrMarker, blockerSeconds, pidFile, parentTail)
}

func withLogCapture(t *testing.T) (*bytes.Buffer, *bool) {
	t.Helper()
	var buf bytes.Buffer
	fatal := false
	logger := log.StandardLogger()
	prevOut := logger.Out
	prevFormatter := logger.Formatter
	prevExit := logger.ExitFunc
	logger.SetOutput(&buf)
	logger.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
	logger.ExitFunc = func(int) { fatal = true }
	t.Cleanup(func() {
		logger.SetOutput(prevOut)
		logger.SetFormatter(prevFormatter)
		logger.ExitFunc = prevExit
	})
	return &buf, &fatal
}

func requireElapsed(t *testing.T, elapsed, min, max time.Duration) {
	t.Helper()
	if elapsed < min || elapsed > max {
		t.Fatalf("elapsed %v, want between %v and %v", elapsed, min, max)
	}
}

func cleanupSleepPID(t *testing.T, pidFile string) {
	t.Helper()
	t.Cleanup(func() {
		data, err := os.ReadFile(pidFile)
		if err != nil {
			return
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		if err != nil || pid <= 1 {
			return
		}
		out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "command=").Output()
		if err != nil || !strings.Contains(string(out), "sleep") {
			return
		}
		proc, err := os.FindProcess(pid)
		if err != nil {
			return
		}
		_ = proc.Kill()
	})
}

func TestCommandCheckerExitCodesAndOutput(t *testing.T) {
	tests := []struct {
		name      string
		script    string
		want      bool
		wantLog   []string
		forbidLog []string
	}{
		{
			name:    "exit 0 is reboot required and keeps output",
			script:  markedExit(0),
			want:    true,
			wantLog: []string{"checking if reboot is required", "stdout" + stdoutMarker, "stderr" + stderrMarker},
		},
		{
			name:      "exit 1 is not reboot required and is not a warning",
			script:    markedExit(1),
			want:      false,
			forbidLog: []string{"unexpected exit code", "timed out", "drain grace"},
		},
		{
			name:    "unexpected exit code warns and is not reboot required",
			script:  markedExit(2),
			want:    false,
			wantLog: []string{"unexpected exit code: 2", "stdout" + stdoutMarker, "stderr" + stderrMarker},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf, fatal := withLogCapture(t)
			checker := CommandChecker{CheckCommand: shell(tt.script), Timeout: time.Second}
			if got := checker.RebootRequired(); got != tt.want {
				t.Fatalf("RebootRequired() = %v, want %v; log=%s", got, tt.want, buf.String())
			}
			if *fatal {
				t.Fatalf("unexpected fatal; log=%s", buf.String())
			}
			logged := buf.String()
			for _, fragment := range tt.wantLog {
				if !strings.Contains(logged, fragment) {
					t.Errorf("log missing %q\n%s", fragment, logged)
				}
			}
			for _, fragment := range tt.forbidLog {
				if strings.Contains(logged, fragment) {
					t.Errorf("log unexpectedly contains %q\n%s", fragment, logged)
				}
			}
		})
	}
}

func TestCommandTimeoutZeroValueFallback(t *testing.T) {
	constructed, err := NewCommandChecker("true", 1, false)
	if err != nil {
		t.Fatal(err)
	}
	if constructed.Timeout != 0 {
		t.Fatalf("constructor Timeout = %v, want zero", constructed.Timeout)
	}
	if got := constructed.commandTimeout(); got != defaultCommandTimeout {
		t.Fatalf("constructor effective timeout = %v, want %v", got, defaultCommandTimeout)
	}
	if !constructed.RebootRequired() {
		t.Fatal("zero Timeout on constructor result failed closed")
	}

	literal := CommandChecker{CheckCommand: []string{"true"}}
	if got := literal.commandTimeout(); got != defaultCommandTimeout {
		t.Fatalf("literal effective timeout = %v, want %v", got, defaultCommandTimeout)
	}
	if !literal.RebootRequired() {
		t.Fatal("zero Timeout on direct struct failed closed")
	}
	literal.Timeout = 5 * time.Second
	if got := literal.commandTimeout(); got != 5*time.Second {
		t.Fatalf("explicit timeout = %v", got)
	}
}

func TestCommandCheckerTimeout(t *testing.T) {
	buf, fatal := withLogCapture(t)
	checker := CommandChecker{CheckCommand: shell(sleepingCommand()), Timeout: shortDeadline}
	start := time.Now()
	got := checker.RebootRequired()
	elapsed := time.Since(start)
	if got {
		t.Fatal("timed out sentinel returned true")
	}
	if *fatal {
		t.Fatal("timeout fatals the checker")
	}
	requireElapsed(t, elapsed, shortDeadline/2, shortDeadline+commandWaitDelay+scheduleSlack)
	logged := buf.String()
	for _, fragment := range []string{
		"timed out after " + shortDeadline.String(),
		"/bin/sh",
		"stdout" + stdoutMarker,
		"stderr" + stderrMarker,
	} {
		if !strings.Contains(logged, fragment) {
			t.Errorf("log missing %q\n%s", fragment, logged)
		}
	}
}

func TestCommandCheckerDescendantDoesNotHang(t *testing.T) {
	t.Run("parent killed while descendant holds pipes", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "descendant.pid")
		cleanupSleepPID(t, pidFile)
		buf, fatal := withLogCapture(t)
		checker := CommandChecker{
			CheckCommand: shell(descendantScript(pidFile, fmt.Sprintf("sleep %d", blockerSeconds))),
			Timeout:      descendantLimit,
		}
		start := time.Now()
		got := checker.RebootRequired()
		elapsed := time.Since(start)
		if got {
			t.Fatal("killed sentinel returned true")
		}
		if *fatal {
			t.Fatal("deadline fatals the checker")
		}
		if _, err := os.Stat(pidFile); err != nil {
			t.Fatalf("descendant was not started: %v", err)
		}
		requireElapsed(t, elapsed, descendantLimit/2, descendantLimit+commandWaitDelay+scheduleSlack)
		if !strings.Contains(buf.String(), "timed out after "+descendantLimit.String()) {
			t.Fatalf("log = %s", buf.String())
		}
	})

	t.Run("parent exits while descendant holds pipes", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "descendant.pid")
		cleanupSleepPID(t, pidFile)
		buf, fatal := withLogCapture(t)
		checker := CommandChecker{
			CheckCommand: shell(descendantScript(pidFile, "exit 0")),
			Timeout:      drainGraceBudget,
		}
		start := time.Now()
		got := checker.RebootRequired()
		elapsed := time.Since(start)
		if got {
			t.Fatal("drain overrun returned true")
		}
		if *fatal {
			t.Fatal("ErrWaitDelay fatals the checker")
		}
		if _, err := os.Stat(pidFile); err != nil {
			t.Fatalf("descendant was not started: %v", err)
		}
		requireElapsed(t, elapsed, commandWaitDelay/2, commandWaitDelay+scheduleSlack)
		logged := buf.String()
		for _, fragment := range []string{"output drain grace", "WaitDelay", "stdout" + stdoutMarker, "stderr" + stderrMarker} {
			if !strings.Contains(logged, fragment) {
				t.Errorf("log missing %q\n%s", fragment, logged)
			}
		}
		if strings.Contains(logged, "timed out") {
			t.Fatalf("drain overrun classified as deadline: %s", logged)
		}
	})
}

func TestCommandCheckerConcurrentAndReuse(t *testing.T) {
	buf, _ := withLogCapture(t)
	var fatal atomic.Bool
	log.StandardLogger().ExitFunc = func(int) { fatal.Store(true) }

	checker := CommandChecker{CheckCommand: shell(sleepingCommand()), Timeout: shortDeadline}
	const workers = 8
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if checker.RebootRequired() {
				errs <- fmt.Errorf("concurrent RebootRequired returned true")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if fatal.Load() {
		t.Fatal("concurrent timeout fatals the checker")
	}

	buf.Reset()
	start := time.Now()
	if checker.RebootRequired() {
		t.Fatal("second timeout call returned true; context was retained")
	}
	requireElapsed(t, time.Since(start), shortDeadline/2, shortDeadline+commandWaitDelay+scheduleSlack)
	if fatal.Load() {
		t.Fatal("second timeout fatals the checker")
	}

	buf.Reset()
	checker.CheckCommand = shell(fmt.Sprintf("echo %s; echo %s >&2", freshStdout, freshStderr))
	if !checker.RebootRequired() {
		t.Fatalf("command after timeout returned false; log=%s", buf.String())
	}
	logged := buf.String()
	if strings.Contains(logged, stdoutMarker) || strings.Contains(logged, stderrMarker) {
		t.Fatalf("retained output from the timed out command: %s", logged)
	}
	for _, fragment := range []string{"stdout" + freshStdout, "stderr" + freshStderr} {
		if !strings.Contains(logged, fragment) {
			t.Errorf("log missing %q\n%s", fragment, logged)
		}
	}
}
