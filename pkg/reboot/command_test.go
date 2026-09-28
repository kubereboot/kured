package reboot

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
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
			got, err := NewCommandRebooter(tt.args.rebootCommand)
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

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	logger := log.StandardLogger()
	prevOut := logger.Out
	prevFormatter := logger.Formatter
	logger.SetOutput(&buf)
	logger.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
	t.Cleanup(func() {
		logger.SetOutput(prevOut)
		logger.SetFormatter(prevFormatter)
	})
	return &buf
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

func TestCommandRebooterZeroTimeoutFallback(t *testing.T) {
	constructed, err := NewCommandRebooter("echo ok")
	if err != nil {
		t.Fatal(err)
	}
	if constructed.Timeout != 0 {
		t.Fatalf("constructor Timeout = %v, want zero", constructed.Timeout)
	}
	if got := constructed.commandTimeout(); got != defaultCommandTimeout {
		t.Fatalf("constructor effective timeout = %v, want %v", got, defaultCommandTimeout)
	}

	literal := CommandRebooter{RebootCommand: []string{"true"}}
	if got := literal.commandTimeout(); got != defaultCommandTimeout {
		t.Fatalf("literal effective timeout = %v, want %v", got, defaultCommandTimeout)
	}
	if err := literal.Reboot(); err != nil {
		t.Fatalf("zero Timeout on direct struct failed closed: %v", err)
	}
	literal.Timeout = 5 * time.Second
	if got := literal.commandTimeout(); got != 5*time.Second {
		t.Fatalf("explicit timeout = %v", got)
	}
}

func TestCommandRebooterExitCodesAndOutput(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		buf := captureLog(t)
		rebooter := CommandRebooter{RebootCommand: shell(markedExit(0)), Timeout: time.Second}
		if err := rebooter.Reboot(); err != nil {
			t.Fatal(err)
		}
		logged := buf.String()
		for _, fragment := range []string{"Invoked reboot command", "stdout" + stdoutMarker, "stderr" + stderrMarker} {
			if !strings.Contains(logged, fragment) {
				t.Errorf("log missing %q\n%s", fragment, logged)
			}
		}
	})

	t.Run("nonzero keeps diagnostics", func(t *testing.T) {
		rebooter := CommandRebooter{RebootCommand: shell(markedExit(3)), Timeout: time.Second}
		err := rebooter.Reboot()
		if err == nil {
			t.Fatal("expected error")
		}
		msg := err.Error()
		for _, fragment := range []string{"error invoking reboot command", "exit status 3", "stdout: " + stdoutMarker, "stderr: " + stderrMarker} {
			if !strings.Contains(msg, fragment) {
				t.Errorf("error missing %q: %s", fragment, msg)
			}
		}
	})

	t.Run("missing executable keeps diagnostics", func(t *testing.T) {
		rebooter := CommandRebooter{RebootCommand: []string{"/nonexistent/kured-reboot-test"}, Timeout: time.Second}
		err := rebooter.Reboot()
		if err == nil {
			t.Fatal("expected error")
		}
		msg := err.Error()
		for _, fragment := range []string{"error invoking reboot command", "/nonexistent/kured-reboot-test", "stdout:", "stderr:"} {
			if !strings.Contains(msg, fragment) {
				t.Errorf("error missing %q: %s", fragment, msg)
			}
		}
	})
}

func TestCommandRebooterTimeout(t *testing.T) {
	rebooter := CommandRebooter{RebootCommand: shell(sleepingCommand()), Timeout: shortDeadline}
	start := time.Now()
	err := rebooter.Reboot()
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	requireElapsed(t, elapsed, shortDeadline/2, shortDeadline+commandWaitDelay+scheduleSlack)
	msg := err.Error()
	for _, fragment := range []string{
		"timed out after " + shortDeadline.String(),
		"/bin/sh",
		"stdout: " + stdoutMarker,
		"stderr: " + stderrMarker,
	} {
		if !strings.Contains(msg, fragment) {
			t.Errorf("error missing %q: %s", fragment, msg)
		}
	}
}

func TestCommandRebooterDescendantDoesNotHang(t *testing.T) {
	t.Run("parent killed while descendant holds pipes", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "descendant.pid")
		cleanupSleepPID(t, pidFile)
		rebooter := CommandRebooter{
			RebootCommand: shell(descendantScript(pidFile, fmt.Sprintf("sleep %d", blockerSeconds))),
			Timeout:       descendantLimit,
		}
		start := time.Now()
		err := rebooter.Reboot()
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if _, statErr := os.Stat(pidFile); statErr != nil {
			t.Fatalf("descendant was not started: %v", statErr)
		}
		requireElapsed(t, elapsed, descendantLimit/2, descendantLimit+commandWaitDelay+scheduleSlack)
		if !strings.Contains(err.Error(), "timed out after "+descendantLimit.String()) {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("parent exits while descendant holds pipes", func(t *testing.T) {
		pidFile := filepath.Join(t.TempDir(), "descendant.pid")
		cleanupSleepPID(t, pidFile)
		rebooter := CommandRebooter{
			RebootCommand: shell(descendantScript(pidFile, "exit 0")),
			Timeout:       drainGraceBudget,
		}
		start := time.Now()
		err := rebooter.Reboot()
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("expected drain error")
		}
		if !errors.Is(err, exec.ErrWaitDelay) {
			t.Fatalf("error = %v, want ErrWaitDelay", err)
		}
		if _, statErr := os.Stat(pidFile); statErr != nil {
			t.Fatalf("descendant was not started: %v", statErr)
		}
		requireElapsed(t, elapsed, commandWaitDelay/2, commandWaitDelay+scheduleSlack)
		msg := err.Error()
		for _, fragment := range []string{"output drain grace", "stdout: " + stdoutMarker, "stderr: " + stderrMarker} {
			if !strings.Contains(msg, fragment) {
				t.Errorf("error missing %q: %s", fragment, msg)
			}
		}
		if strings.Contains(msg, "timed out") {
			t.Fatalf("drain overrun classified as deadline: %s", msg)
		}
	})
}

func TestCommandRebooterConcurrentAndReuse(t *testing.T) {
	buf := captureLog(t)
	rebooter := CommandRebooter{RebootCommand: shell(sleepingCommand()), Timeout: shortDeadline}
	const workers = 4
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := rebooter.Reboot()
			if err == nil || !strings.Contains(err.Error(), "timed out after") {
				errs <- fmt.Errorf("concurrent Reboot() = %v", err)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	start := time.Now()
	err := rebooter.Reboot()
	if err == nil || !strings.Contains(err.Error(), "timed out after "+shortDeadline.String()) {
		t.Fatalf("second call = %v", err)
	}
	requireElapsed(t, time.Since(start), shortDeadline/2, shortDeadline+commandWaitDelay+scheduleSlack)

	buf.Reset()
	rebooter.RebootCommand = shell(fmt.Sprintf("echo %s; echo %s >&2", freshStdout, freshStderr))
	if err := rebooter.Reboot(); err != nil {
		t.Fatal(err)
	}
	logged := buf.String()
	if strings.Contains(logged, stdoutMarker) || strings.Contains(logged, stderrMarker) {
		t.Fatalf("retained output from the timed out command: %s", logged)
	}
	if !strings.Contains(logged, "stdout"+freshStdout) || !strings.Contains(logged, "stderr"+freshStderr) {
		t.Fatalf("log = %s", logged)
	}
}
