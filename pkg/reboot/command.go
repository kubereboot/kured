package reboot

import (
	"github.com/kubereboot/kured/pkg/util"
	log "github.com/sirupsen/logrus"
)

type commandRebootMethod struct {
	nodeID        string
	rebootCommand []string
}

func NewCommandReboot(nodeID string, rebootCommand []string) *commandRebootMethod {
	return &commandRebootMethod{nodeID: nodeID, rebootCommand: rebootCommand}
}

func (c *commandRebootMethod) Reboot() {
	log.Infof("Running command: %s for node: %s", c.rebootCommand, c.nodeID)
	if err := util.NewCommand(c.rebootCommand[0], c.rebootCommand[1:]...).Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}
