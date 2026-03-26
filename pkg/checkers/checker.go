// Package checkers provide interfaces and implementations for determining
// whether a system reboot is required. It includes checkers based on file
// presence or custom commands and supports privileged command execution
// in containerized environments. These checkers are used by kured to
// detect conditions that should trigger node reboots.
// You can use that package if you fork Kured's main loop.
package checkers

import (
	"fmt"
	"log/slog"
)

// Checker is the standard interface to use to check
// if a reboot is required. Its types must implement a
// CheckRebootRequired method which returns a single boolean
// clarifying whether a reboot is expected or not.
type Checker interface {
	RebootRequired() bool
}

// NewRebootChecker validates the rebootSentinelCommand, rebootSentinelFile input,
// then chains to the right constructor.
func NewRebootChecker(rebootSentinelCommand string, rebootSentinelFile string) (Checker, error) {
	// An override of rebootSentinelCommand means a privileged command
	if rebootSentinelCommand != "" {
		slog.Info(fmt.Sprintf("Sentinel checker is (privileged) user provided command: %s", rebootSentinelCommand))
		return NewCommandChecker(rebootSentinelCommand, 1, true)
	}
	slog.Info("Sentinel checker is (unprivileged) testing for the presence of the file: " + rebootSentinelFile)
	return NewFileRebootChecker(rebootSentinelFile)
}
