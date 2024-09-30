package reboot

import (
	"github.com/kubereboot/kured/pkg/util"
	log "github.com/sirupsen/logrus"
)

// CommandRebooter holds context-information for a command reboot.
type CommandRebooter struct {
	RebootCommand []string
}

// Reboot triggers the reboot command
func (c CommandRebooter) Reboot() {
	log.Infof("Invoking command: %s", c.RebootCommand)
	if err := util.NewCommand(c.RebootCommand[0], c.RebootCommand[1:]...).Run(); err != nil {
		log.Fatalf("Error invoking reboot command: %v", err)
	}
}
