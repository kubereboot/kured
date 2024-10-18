package reboot

import (
	"fmt"
	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"os/exec"
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

	log.Infof("Invoking command: %s", c.RebootCommand)
	cmd := exec.Command(c.RebootCommand[0], c.RebootCommand[1:]...)
	cmd.Stdout = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "out").
		WriterLevel(log.InfoLevel)

	cmd.Stderr = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "err").
		WriterLevel(log.WarnLevel)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error invoking reboot command %v", err)
	}
	return nil
}

// NewCommandRebooter is the constructor to create a CommandRebooter from a string not
// yet shell lexed. You can skip this constructor if you parse the data correctly first
// when instantiating a CommandRebooter instance.
func NewCommandRebooter(rebootCommand string, rebootDelay time.Duration, privileged bool, pid int) *CommandRebooter {
	cmd, err := shlex.Split(rebootCommand)
	if err != nil {
		log.Fatalf("Error parsing provided reboot command: %v", err)
	}

	if privileged {
		if pid < 1 {
			log.Fatalf("Incorrect PID number")
		}
		cmdline := []string{"/usr/bin/nsenter", fmt.Sprintf("-m/proc/%d/ns/mnt", pid), "--"}
		cmdline = append(cmdline, cmd...)

		return &CommandRebooter{
			RebootCommand: cmdline,
			GenericRebooter: GenericRebooter{
				RebootDelay: rebootDelay,
			},
		}
	}
	return &CommandRebooter{
		RebootCommand: cmd,
		GenericRebooter: GenericRebooter{
			RebootDelay: rebootDelay,
		},
	}
}
