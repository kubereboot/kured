package reboot

import (
	"os"
	"syscall"

	log "github.com/sirupsen/logrus"
)

// SignalRebooter holds context-information for a signal reboot.
type SignalRebooter struct {
	NodeID string
	Signal int
}

// Reboot triggers the reboot signal using SIGTERMIN+5
func (c SignalRebooter) Reboot() {
	log.Infof("Emit reboot-signal for node: %s", c.NodeID)

	process, err := os.FindProcess(1)
	if err != nil {
		log.Fatalf("There was no systemd process found: %v", err)
	}

	err = process.Signal(syscall.Signal(c.Signal))
	if err != nil {
		log.Fatalf("Signal of SIGRTMIN+5 failed: %v", err)
	}
}
