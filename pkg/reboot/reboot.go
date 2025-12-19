// Package reboot provides mechanisms to trigger node reboots using different
// methods, like custom commands or signals.
// Each of those includes constructors and interfaces for handling different reboot
// strategies, supporting privileged command execution via nsenter for containerized environments.
package reboot

import (
	"fmt"
	"log/slog"
	"time"
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
func NewRebooter(rebootMethod string, rebootCommand string, rebootSignal int, rebootDelay time.Duration, privileged bool, pid int) (Rebooter, error) {
	switch rebootMethod {
	case "command":
		slog.Info("Will reboot using command", "cmd", rebootCommand)
		return NewCommandRebooter(rebootCommand, rebootDelay, privileged, pid)
	case "signal":
		slog.Info("Will reboot using signal", "signal", rebootSignal)
		return NewSignalRebooter(rebootSignal, rebootDelay)
	default:
		return nil, fmt.Errorf("invalid reboot-method configured %s, expected signal or command", rebootMethod)
	}
}

// GenericRebooter intent was to implement standard features for reboots
// It currently only implements the tools for delaying reboots.
type GenericRebooter struct {
	RebootDelay time.Duration
}

// DelayReboot will delay the reboot for the configured time
func (g GenericRebooter) DelayReboot() {
	if g.RebootDelay > 0 {
		slog.Debug(fmt.Sprintf("Delayed reboot for %s", g.RebootDelay))
		time.Sleep(g.RebootDelay)
	}
}
