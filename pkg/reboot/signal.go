package reboot

import (
	"fmt"
	"os"
	"syscall"
)

// SignalRebooter holds context-information for a signal reboot.
type SignalRebooter struct {
	Signal int
}

// Reboot triggers the reboot signal
func (c SignalRebooter) Reboot() error {
	process, err := os.FindProcess(1)
	if err != nil {
		return fmt.Errorf("not running on Unix: %v", err)
	}

	err = process.Signal(syscall.Signal(c.Signal))
	// Either PID does not exist, or the signal does not work. Hoping for
	// a decent enough error.
	if err != nil {
		return fmt.Errorf("signal of SIGRTMIN+5 failed: %v", err)
	}
	return nil
}

// NewSignalRebooter is the constructor which sets the signal number.
// The constructor does not yet validate any input. It should be done in a later commit.
func NewSignalRebooter(sig int) (*SignalRebooter, error) {
	if sig < 1 {
		return nil, fmt.Errorf("invalid signal: %v", sig)
	}
	return &SignalRebooter{Signal: sig}, nil
}
