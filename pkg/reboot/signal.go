package reboot

import (
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// SignalRebootMethod holds context-information for a signal reboot.
type SignalRebootMethod struct {
	nodeID string
	signal int
}

// NewSignalReboot creates a new signal-rebooter which can run unprivileged.
func NewSignalReboot(nodeID string, signal int) *SignalRebootMethod {
	return &SignalRebootMethod{nodeID: nodeID, signal: signal}
}

// Reboot triggers the signal-reboot.
func (c *SignalRebootMethod) Reboot() {
	log.Infof("Emit reboot-signal for node: %s", c.nodeID)

	process, err := os.FindProcess(1)
	if err != nil {
		log.Fatalf("There was no systemd process found: %v", err)
	}

	err = process.Signal(syscall.Signal(c.signal))
	if err != nil {
		log.Fatalf("Signal of SIGRTMIN+5 failed: %v", err)
	}
}
