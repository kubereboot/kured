package util

import (
	"fmt"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

// NewCommand creates a new Command with stdout/stderr wired to our standard logger
func NewCommand(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "out").
		WriterLevel(log.InfoLevel)

	cmd.Stderr = log.NewEntry(log.StandardLogger()).
		WithField("cmd", cmd.Args[0]).
		WithField("std", "err").
		WriterLevel(log.WarnLevel)

	return cmd
}

// PrivilegedHostCommand wraps the command with nsenter.
// It allows to run a command from systemd's namespace for example (pid 1)
// This relies on hostPID:true and privileged:true to enter host mount space
// For info, rancher based need different pid, which should be user given.
// until we have a better discovery mechanism.
func PrivilegedHostCommand(pid int, command []string) []string {
	cmd := []string{"/usr/bin/nsenter", fmt.Sprintf("-m/proc/%d/ns/mnt", pid), "--"}
	cmd = append(cmd, command...)
	return cmd
}
