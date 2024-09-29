package checkers

import (
	"github.com/kubereboot/kured/pkg/util"
	log "github.com/sirupsen/logrus"
	"os/exec"
)

type Checker interface {
	CheckRebootRequired() bool
}

// UnprivilegedRebootChecker is the default reboot checker.
// It is unprivileged, and tests the presence of a files
type UnprivilegedRebootChecker struct {
	CheckCommand []string
}

// CheckRebootRequired runs the test command of the file
// needs refactoring to also return an error, instead of leaking it inside the code.
// This needs refactoring to get rid of NewCommand
// This needs refactoring to only contain file location, instead of CheckCommand
func (rc UnprivilegedRebootChecker) CheckRebootRequired() bool {
	cmd := util.NewCommand(rc.CheckCommand[0], rc.CheckCommand[1:]...)
	if err := cmd.Run(); err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
			// We assume a non-zero exit code means 'reboot not required', but of course
			// the user could have misconfigured the sentinel command or something else
			// went wrong during its execution. In that case, not entering a reboot loop
			// is the right thing to do, and we are logging stdout/stderr of the command
			// so it should be obvious what is wrong.
			if cmd.ProcessState.ExitCode() != 1 {
				log.Warnf("sentinel command ended with unexpected exit code: %v", cmd.ProcessState.ExitCode())
			}
			return false
		default:
			// Something was grossly misconfigured, such as the command path being wrong.
			log.Fatalf("Error invoking sentinel command: %v", err)
		}
	}
	return true
}

// NsEnterRebootChecker is using a custom command to check
// if a reboot is required, but therefore needs a pid for entering the namespace,
// on top of the required command. This requires elevation.
type NsEnterRebootChecker struct {
	CustomCheckCommand []string
	NamespacePid       int
}

func (rc NsEnterRebootChecker) CheckRebootRequired() bool {
	privCommand := util.PrivilegedHostCommand(rc.NamespacePid, rc.CustomCheckCommand)
	cmd := util.NewCommand(privCommand[0], privCommand[1:]...)
	if err := cmd.Run(); err != nil {
		switch err := err.(type) {
		case *exec.ExitError:
			// We assume a non-zero exit code means 'reboot not required', but of course
			// the user could have misconfigured the sentinel command or something else
			// went wrong during its execution. In that case, not entering a reboot loop
			// is the right thing to do, and we are logging stdout/stderr of the command
			// so it should be obvious what is wrong.
			if cmd.ProcessState.ExitCode() != 1 {
				log.Warnf("sentinel command ended with unexpected exit code: %v", cmd.ProcessState.ExitCode())
			}
			return false
		default:
			// Something was grossly misconfigured, such as the command path being wrong.
			log.Fatalf("Error invoking sentinel command: %v", err)
		}
	}
	return true
}
