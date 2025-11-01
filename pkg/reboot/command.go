package reboot

import (
	"bytes"
	"fmt"
	"github.com/google/shlex"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// CommandRebooter holds context-information for a reboot with command
type CommandRebooter struct {
	RebootCommand []string
	GenericRebooter
}

// Reboot triggers the reboot command
func (c CommandRebooter) Reboot() error {
	c.DelayReboot()
	slog.Info("Invoking reboot command", "cmd", c.RebootCommand)

	bufStdout := new(bytes.Buffer)
	bufStderr := new(bytes.Buffer)
	cmd := exec.Command(c.RebootCommand[0], c.RebootCommand[1:]...) // #nosec G204
	cmd.Stdout = bufStdout
	cmd.Stderr = bufStderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error invoking reboot command %s: %v (stdout: %v, stderr: %v)", c.RebootCommand, err, bufStdout.String(), bufStderr.String())
	}
	slog.Info("Invoked reboot command", "cmd", strings.Join(cmd.Args, " "), "stdout", bufStdout.String(), "stderr", bufStderr.String())
	return nil
}

// NewCommandRebooter is the constructor to create a CommandRebooter from a string not
// yet shell lexed. You can skip this constructor if you parse the data correctly first
// when instantiating a CommandRebooter instance.
func NewCommandRebooter(rebootCommand string, rebootDelay time.Duration, privileged bool, pid int) (*CommandRebooter, error) {
	if rebootCommand == "" {
		return nil, fmt.Errorf("no reboot command specified")
	}
	cmd := []string{}
	if privileged && pid > 0 {
		cmd = append(cmd, "/usr/bin/nsenter", fmt.Sprintf("-m/proc/%d/ns/mnt", pid), "--")
	}

	parsedCommand, err := shlex.Split(rebootCommand)
	if err != nil {
		return nil, fmt.Errorf("error %v when parsing reboot command %s", err, rebootCommand)
	}

	cmd = append(cmd, parsedCommand...)
	return &CommandRebooter{
		RebootCommand: cmd,
		GenericRebooter: GenericRebooter{
			RebootDelay: rebootDelay,
		},
	}, nil
}
