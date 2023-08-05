package reboot

import (
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
)

type signalRebootMethod struct {
	nodeID string
}

func NewSignalReboot(nodeID string) *signalRebootMethod {
	return &signalRebootMethod{nodeID: nodeID}
}

func (c *signalRebootMethod) Reboot() {
	log.Infof("Emit reboot-signal for node: %s", c.nodeID)

	process, err := os.FindProcess(1)
	if err != nil {
		log.Fatalf("There was no systemd process found: %v", err)
	}

	err = process.Signal(syscall.Signal(34 + 5)) // SIGRTMIN+5
	if err != nil {
		log.Fatalf("Signal of SIGRTMIN+5 failed: %v", err)
	}
}
