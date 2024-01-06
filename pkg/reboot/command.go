package reboot

import (
	"github.com/kubereboot/kured/pkg/util"
	log "github.com/sirupsen/logrus"
)

// CommandRebootMethod holds context-information for a command reboot.
type CommandRebootMethod struct {
	nodeID        string
	rebootCommand []string
}

// NewCommandReboot creates a new command-rebooter which needs full privileges on the host.
func NewCommandReboot(nodeID string, rebootCommand []string) *CommandRebootMethod {
	return &CommandRebootMethod{nodeID: nodeID, rebootCommand: rebootCommand}
}

// Reboot triggers the command-reboot.
func (c *CommandRebootMethod) Reboot() {
	log.Infof("Running command: %s for node: %s", c.rebootCommand, c.nodeID)
	if err := util.NewCommand(c.rebootCommand[0], c.rebootCommand[1:]...).Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}
