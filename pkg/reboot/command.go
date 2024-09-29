package reboot

import (
	"github.com/kubereboot/kured/pkg/util"
	log "github.com/sirupsen/logrus"
)

// CommandRebooter holds context-information for a command reboot.
type CommandRebooter struct {
	NodeID        string
	RebootCommand []string
}

// NewCommandReboot creates a new command-rebooter which needs full privileges on the host.

// Reboot triggers the command-reboot.
func (c CommandRebooter) Reboot() {
	log.Infof("Running command: %s for node: %s", c.RebootCommand, c.NodeID)
	if err := util.NewCommand(c.RebootCommand[0], c.RebootCommand[1:]...).Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}
