package reboot

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

// Rebooter is the standard interface to use to execute
// the reboot, after it has been considered as necessary.
// The Reboot method does not expect any return, yet should
// most likely be refactored in the future to return an error
type Rebooter interface {
	Reboot()
}

// NewRebooter validates the rebootMethod, rebootCommand, and rebootSignal input,
// then chains to the right constructor. This can be made internal later, as
// only the real rebooters constructors should be public (by opposition to this one)
func NewRebooter(rebootMethod string, rebootCommand string, rebootSignal int) (Rebooter, error) {
	switch {
	case rebootMethod == "command":
		log.Infof("Reboot command: %s", rebootCommand)
		return NewCommandRebooter(rebootCommand)
	case rebootMethod == "signal":
		log.Infof("Reboot signal: %d", rebootSignal)
		return NewSignalRebooter(rebootSignal)
	default:
		return nil, fmt.Errorf("invalid reboot-method configured %s, expected signal or command", rebootMethod)
	}
}
