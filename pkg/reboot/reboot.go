package reboot

import (
	"fmt"
	"github.com/sirupsen/logrus"
)

// Rebooter is the standard interface to use to execute
// the reboot, after it has been considered as necessary.
// The Reboot method does not expect any return, yet should
// most likely be refactored in the future to return an error
type Rebooter interface {
	Reboot() error
}

// NewRebooter validates the rebootMethod, rebootCommand, and rebootSignal input,
// then chains to the right constructor.
func NewRebooter(rebootMethod string, rebootCommand string, rebootSignal int) (Rebooter, error) {
	switch {
	case rebootMethod == "command":
		logrus.Infof("Reboot command: %s", rebootCommand)
		return NewCommandRebooter(rebootCommand)
	case rebootMethod == "signal":
		logrus.Infof("Reboot signal: %d", rebootSignal)
		return NewSignalRebooter(rebootSignal)
	default:
		return nil, fmt.Errorf("invalid reboot-method configured %s, expected signal or command", rebootMethod)
	}
}
