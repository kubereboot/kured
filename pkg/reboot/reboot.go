package reboot

import (
	log "github.com/sirupsen/logrus"
	"time"
)

// Rebooter is the standard interface to use to execute
// the reboot, after it has been considered as necessary.
// The Reboot method does not expect any return, yet should
// most likely be refactored in the future to return an error
type Rebooter interface {
	Reboot() error
}

type GenericRebooter struct {
	RebootDelay time.Duration
}

func (g GenericRebooter) DelayReboot() {
	if g.RebootDelay > 0 {
		log.Infof("Delayed reboot for %s", g.RebootDelay)
		time.Sleep(g.RebootDelay)
	}
}
