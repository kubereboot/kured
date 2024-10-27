package internal

import (
	"fmt"
	"github.com/kubereboot/kured/pkg/checkers"
	"github.com/kubereboot/kured/pkg/reboot"
	log "github.com/sirupsen/logrus"
)

// NewRebooter validates the rebootMethod, rebootCommand, and rebootSignal input,
// then chains to the right constructor.
func NewRebooter(rebootMethod string, rebootCommand string, rebootSignal int) (reboot.Rebooter, error) {
	switch {
	case rebootMethod == "command":
		log.Infof("Reboot command: %s", rebootCommand)
		return reboot.NewCommandRebooter(rebootCommand)
	case rebootMethod == "signal":
		log.Infof("Reboot signal: %d", rebootSignal)
		return reboot.NewSignalRebooter(rebootSignal)
	default:
		return nil, fmt.Errorf("invalid reboot-method configured %s, expected signal or command", rebootMethod)
	}
}

// NewRebootChecker validates the rebootSentinelCommand, rebootSentinelFile input,
// then chains to the right constructor.
func NewRebootChecker(rebootSentinelCommand string, rebootSentinelFile string) (checkers.Checker, error) {
	// An override of rebootSentinelCommand means a privileged command
	if rebootSentinelCommand != "" {
		log.Infof("Sentinel checker is (privileged) user provided command: %s", rebootSentinelCommand)
		return checkers.NewCommandChecker(rebootSentinelCommand)
	}
	log.Infof("Sentinel checker is (unprivileged) testing for the presence of: %s", rebootSentinelFile)
	return checkers.NewFileRebootChecker(rebootSentinelFile)
}
