package reboot

import (
	"bytes"
	"fmt"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"os/exec"
)

// CommandRebooter holds context-information for a reboot with command
type CommandRebooter struct {
	RebootCommand []string
}

// Reboot triggers the reboot command
func (c CommandRebooter) Reboot() error {
	log.Infof("Invoking command: %s", c.RebootCommand)

	bufStdout := new(bytes.Buffer)
	bufStderr := new(bytes.Buffer)
	cmd := exec.Command(c.RebootCommand[0], c.RebootCommand[1:]...)
	cmd.Stdout = bufStdout
	cmd.Stderr = bufStderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error invoking reboot command %s: %v", c.RebootCommand, err)
	}
	log.Info("Invoked reboot command", "cmd", cmd.Args, "stdout", bufStdout, "stderr", bufStderr)
	return nil
}

// NewCommandRebooter is the constructor to create a CommandRebooter from a string not
// yet shell lexed. You can skip this constructor if you parse the data correctly first
// when instantiating a CommandRebooter instance.
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
