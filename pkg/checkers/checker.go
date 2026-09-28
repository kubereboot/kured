// Package checkers provides interfaces and implementations for determining
// whether a system reboot is required. It includes checkers based on file
// presence or custom commands, and supports privileged command execution
// in containerized environments. These checkers are used by kured to
// detect conditions that should trigger node reboots.
// You can use that package if you fork Kured's main loop.
package checkers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
)

const (
	// defaultCommandTimeout is the deadline used when Timeout is unset.
	// It matches the --sentinel-command-timeout CLI default so existing
	// constructors and direct struct literals stay bounded.
	defaultCommandTimeout = 60 * time.Second
	// commandWaitDelay bounds draining of inherited stdout/stderr pipes after
	// the process exits or its deadline fires. It does not terminate
	// descendant processes. The CLI help text documents this 1s drain grace.
	commandWaitDelay = time.Second
)

// Checker is the standard interface to use to check
// if a reboot is required. Its types must implement a
// CheckRebootRequired method which returns a single boolean
// clarifying whether a reboot is expected or not.
type Checker interface {
	RebootRequired() bool
}

// FileRebootChecker is the default reboot checker.
// It is unprivileged, and tests the presence of a files
type FileRebootChecker struct {
	FilePath string
}

// RebootRequired checks the file presence
// needs refactoring to also return an error, instead of leaking it inside the code.
// This needs refactoring to get rid of NewCommand
// This needs refactoring to only contain file location, instead of CheckCommand
func (rc FileRebootChecker) RebootRequired() bool {
	if _, err := os.Stat(rc.FilePath); err == nil {
		return true
	}
	return false
}

// NewFileRebootChecker is the constructor for the file based reboot checker
// TODO: Add extra input validation on filePath string here
func NewFileRebootChecker(filePath string) (*FileRebootChecker, error) {
	return &FileRebootChecker{
		FilePath: filePath,
	}, nil
}

// CommandChecker is using a custom command to check
// if a reboot is required. There are two modes of behaviour,
// if Privileged is granted, the NamespacePid is used to nsenter
// the given PID's namespace.
type CommandChecker struct {
	CheckCommand []string
	NamespacePid int
	Privileged   bool
	// Timeout is the deadline for a single RebootRequired execution.
	// The zero value falls back to 60s. Each call creates its own context
	// and output buffers; nothing about a previous run is retained.
	Timeout time.Duration
}

// commandTimeout returns the deadline for one execution.
func (rc CommandChecker) commandTimeout() time.Duration {
	if rc.Timeout == 0 {
		return defaultCommandTimeout
	}
	return rc.Timeout
}

// RebootRequired for CommandChecker runs a command without returning
// any eventual error. This should be later refactored to return the errors,
// instead of logging and fataling them here.
// Deadline expiration and an output-drain overrun are warnings that return
// false; they do not fatal the process.
func (rc CommandChecker) RebootRequired() bool {
	bufStdout := new(bytes.Buffer)
	bufStderr := new(bytes.Buffer)
	timeout := rc.commandTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// #nosec G204 -- CheckCommand is controlled and validated internally
	cmd := exec.CommandContext(ctx, rc.CheckCommand[0], rc.CheckCommand[1:]...)
	cmd.Stdout = bufStdout
	cmd.Stderr = bufStderr
	cmd.WaitDelay = commandWaitDelay

	if err := cmd.Run(); err != nil {
		// Deadline expiration surfaces as an ExitError from the killed process,
		// so it has to be detected before ordinary exit-code handling.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			log.Warn(fmt.Sprintf("sentinel command timed out after %v", timeout), "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
			return false
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			log.Warn(fmt.Sprintf("sentinel command exceeded output drain grace: %v", err), "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
			return false
		}
		switch err := err.(type) {
		case *exec.ExitError:
			// We assume a non-zero exit code means 'reboot not required', but of course
			// the user could have misconfigured the sentinel command or something else
			// went wrong during its execution. In that case, not entering a reboot loop
			// is the right thing to do, and we are logging stdout/stderr of the command
			// so it should be obvious what is wrong.
			if cmd.ProcessState.ExitCode() != 1 {
				log.Warn(fmt.Sprintf("sentinel command ended with unexpected exit code: %v", cmd.ProcessState.ExitCode()), "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
			}
			return false
		default:
			// Something was grossly misconfigured, such as the command path being wrong.
			log.Fatal(fmt.Sprintf("Error invoking sentinel command: %v", err), "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
		}
	}
	log.Info("checking if reboot is required", "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
	return true
}

// NewCommandChecker is the constructor for the commandChecker, and by default
// runs new commands in a privileged fashion.
// Privileged means wrapping the command with nsenter.
// It allows to run a command from systemd's namespace for example (pid 1)
// This relies on hostPID:true and privileged:true to enter host mount space
// For info, rancher based need different pid, which should be user given.
// until we have a better discovery mechanism.
// Timeout is left zero so existing callers keep the 60s execution default.
func NewCommandChecker(sentinelCommand string, pid int, privileged bool) (*CommandChecker, error) {
	var cmd []string
	if privileged {
		cmd = append(cmd, "/usr/bin/nsenter", fmt.Sprintf("-m/proc/%d/ns/mnt", pid), "--")
	}
	parsedCommand, err := shlex.Split(sentinelCommand)
	if err != nil {
		return nil, fmt.Errorf("error parsing provided sentinel command: %v", err)
	}
	cmd = append(cmd, parsedCommand...)
	return &CommandChecker{
		CheckCommand: cmd,
		NamespacePid: pid,
		Privileged:   privileged,
	}, nil
}
