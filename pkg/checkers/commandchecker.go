package checkers

import (
	"bytes"
	"fmt"
	"github.com/google/shlex"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// CommandChecker is using a custom command to check
// if a reboot is required. There are two modes of behaviour,
// if Privileged is granted, the NamespacePid is used to nsenter
// the given PID's namespace.
type CommandChecker struct {
	CheckCommand []string
	NamespacePid int
	Privileged   bool
}

var exitFunc = os.Exit

// RebootRequired for CommandChecker runs a command without returning
// any eventual error. This should be later refactored to return the errors,
// instead of logging and fataling them here.
func (rc CommandChecker) RebootRequired() bool {
	bufStdout := new(bytes.Buffer)
	bufStderr := new(bytes.Buffer)
	// #nosec G204 -- CheckCommand is controlled and validated internally
	cmd := exec.Command(rc.CheckCommand[0], rc.CheckCommand[1:]...)
	cmd.Stdout = bufStdout
	cmd.Stderr = bufStderr

	if err := cmd.Run(); err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
			// We assume a non-zero exit code means 'reboot not required', but of course
			// the user could have misconfigured the sentinel command or something else
			// went wrong during its execution. In that case, not entering a reboot loop
			// is the right thing to do, and we are logging stdout/stderr of the command
			// so it should be obvious what is wrong.
			if cmd.ProcessState.ExitCode() != 1 {
				slog.Info(fmt.Sprintf("sentinel command ended with unexpected exit code: %v, preventing reboot", cmd.ProcessState.ExitCode()), "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
			}
			return false
		default:
			// Something was grossly misconfigured, such as the command path being wrong.
			slog.Error(fmt.Sprintf("Error invoking sentinel command: %v", err), "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
			// exitFunc is in indirection to help testing
			exitFunc(11)
		}
	}
	slog.Debug("reboot is required", "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
	return true
}

// NewCommandChecker is the constructor for the commandChecker, and by default
// runs new commands in a privileged fashion.
// Privileged means wrapping the command with nsenter.
// It allows to run a command from systemd's namespace for example (pid 1)
// This relies on hostPID:true and privileged:true to enter host mount space
// For info, rancher based need different pid, which should be user given.
// until we have a better discovery mechanism.
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
