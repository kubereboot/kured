package reboot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
)

const (
	// defaultCommandTimeout is the deadline used when Timeout is unset.
	// It matches the --reboot-command-timeout CLI default so existing
	// constructors and direct struct literals stay bounded.
	defaultCommandTimeout = 60 * time.Second
	// commandWaitDelay bounds draining of inherited stdout/stderr pipes after
	// the process exits or its deadline fires. It does not terminate
	// descendant processes. The CLI help text documents this 1s drain grace.
	commandWaitDelay = time.Second
)

// CommandRebooter holds context-information for a reboot with command
type CommandRebooter struct {
	RebootCommand []string
	// Timeout is the deadline for a single Reboot execution.
	// The zero value falls back to 60s. Each call creates its own context
	// and output buffers; nothing about a previous run is retained.
	Timeout time.Duration
}

// commandTimeout returns the deadline for one execution.
func (c CommandRebooter) commandTimeout() time.Duration {
	if c.Timeout == 0 {
		return defaultCommandTimeout
	}
	return c.Timeout
}

// Reboot triggers the reboot command.
// A non-nil error, including deadline expiration, keeps the caller's
// fatal/restart path in force.
func (c CommandRebooter) Reboot() error {
	log.Infof("Invoking command: %s", c.RebootCommand)

	bufStdout := new(bytes.Buffer)
	bufStderr := new(bytes.Buffer)
	timeout := c.commandTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.RebootCommand[0], c.RebootCommand[1:]...) // #nosec G204
	cmd.Stdout = bufStdout
	cmd.Stderr = bufStderr
	cmd.WaitDelay = commandWaitDelay

	if err := cmd.Run(); err != nil {
		// Deadline expiration surfaces as an ExitError from the killed process,
		// so it has to be detected before ordinary exit handling.
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("reboot command %s timed out after %v (stdout: %v, stderr: %v): %w", c.RebootCommand, timeout, bufStdout.String(), bufStderr.String(), err)
		}
		if errors.Is(err, exec.ErrWaitDelay) {
			return fmt.Errorf("reboot command %s exceeded output drain grace (stdout: %v, stderr: %v): %w", c.RebootCommand, bufStdout.String(), bufStderr.String(), err)
		}
		return fmt.Errorf("error invoking reboot command %s: %v (stdout: %v, stderr: %v)", c.RebootCommand, err, bufStdout.String(), bufStderr.String())
	}
	log.Info("Invoked reboot command", "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
	return nil
}

// NewCommandRebooter is the constructor to create a CommandRebooter from a string not
// yet shell lexed. You can skip this constructor if you parse the data correctly first
// when instantiating a CommandRebooter instance.
// Timeout is left zero so existing callers keep the 60s execution default.
func NewCommandRebooter(rebootCommand string) (*CommandRebooter, error) {
	if rebootCommand == "" {
		return nil, fmt.Errorf("no reboot command specified")
	}
	cmd := []string{"/usr/bin/nsenter", fmt.Sprintf("-m/proc/%d/ns/mnt", 1), "--"}
	parsedCommand, err := shlex.Split(rebootCommand)
	if err != nil {
		return nil, fmt.Errorf("error %v when parsing reboot command %s", err, rebootCommand)
	}
	cmd = append(cmd, parsedCommand...)
	return &CommandRebooter{RebootCommand: cmd}, nil
}
