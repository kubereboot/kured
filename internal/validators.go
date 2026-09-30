// Package internal provides convenient tools which shouldn't be in cmd/main
// It will eventually provide internal validation and chaining logic to select
// appropriate reboot and sentinel check methods based on configuration.
// It validates user input and instantiates the correct checker and rebooter implementations
// for use elsewhere in kured.
package internal

import (
	"fmt"
	"time"

	"github.com/kubereboot/kured/pkg/checkers"
	"github.com/kubereboot/kured/pkg/reboot"
	log "github.com/sirupsen/logrus"
)

// NewRebooter validates the rebootMethod, rebootCommand, and rebootSignal input,
// then chains to the right constructor.
// rebootCommandTimeout is stored on command rebooters and must already be
// positive; a zero value is not rewritten here. Signal rebooting is unchanged.
func NewRebooter(rebootMethod string, rebootCommand string, rebootSignal int, rebootCommandTimeout time.Duration) (reboot.Rebooter, error) {
	switch rebootMethod {
	case "command":
		log.Infof("Reboot command: %s", rebootCommand)
		rebooter, err := reboot.NewCommandRebooter(rebootCommand)
		if err != nil {
			return nil, err
		}
		rebooter.Timeout = rebootCommandTimeout
		return rebooter, nil
	case "signal":
		log.Infof("Reboot signal: %d", rebootSignal)
		return reboot.NewSignalRebooter(rebootSignal)
	default:
		return nil, fmt.Errorf("invalid reboot-method configured %s, expected signal or command", rebootMethod)
	}
}

// NewRebootChecker validates the rebootSentinelCommand, rebootSentinelFile input,
// then chains to the right constructor.
// sentinelCommandTimeout is stored on command checkers and must already be
// positive; a zero value is not rewritten here. File checking is unchanged.
func NewRebootChecker(rebootSentinelCommand string, rebootSentinelFile string, sentinelCommandTimeout time.Duration) (checkers.Checker, error) {
	// An override of rebootSentinelCommand means a privileged command
	if rebootSentinelCommand != "" {
		log.Infof("Sentinel checker is (privileged) user provided command: %s", rebootSentinelCommand)
		checker, err := checkers.NewCommandChecker(rebootSentinelCommand, 1, true)
		if err != nil {
			return nil, err
		}
		checker.Timeout = sentinelCommandTimeout
		return checker, nil
	}
	log.Infof("Sentinel checker is (unprivileged) testing for the presence of: %s", rebootSentinelFile)
	return checkers.NewFileRebootChecker(rebootSentinelFile)
}
