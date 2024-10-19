package checkers

import (
	"fmt"
	"github.com/google/shlex"
	"github.com/kubereboot/kured/pkg/util"
	log "github.com/sirupsen/logrus"
	"os"
	"os/exec"
)

// Checker is the standard interface to use to check
// if a reboot is required. Its types must implement a
// CheckRebootRequired method which returns a single boolean
// clarifying whether a reboot is expected or not.
type Checker interface {
	RebootRequired() bool
}

// FileRebootChecker is the default reboot checker.
// It is unprivileged, and tests the presence of a files
type FileRebootChecker struct {
	FilePath string
}

func NewRebootChecker(rebootSentinelCommand string, rebootSentinelFile string) (Checker, error) {
	// An override of rebootSentinelCommand means a privileged command
	if rebootSentinelCommand != "" {
		log.Infof("Sentinel checker is (privileged) user provided command: %s", rebootSentinelCommand)
		return NewCommandChecker(rebootSentinelCommand)
	}
	log.Infof("Sentinel checker is (unprivileged) testing for the presence of: %s", rebootSentinelFile)
	return NewFileRebootChecker(rebootSentinelFile)
}

// RebootRequired checks the file presence
// needs refactoring to also return an error, instead of leaking it inside the code.
// This needs refactoring to get rid of NewCommand
// This needs refactoring to only contain file location, instead of CheckCommand
func (rc FileRebootChecker) RebootRequired() bool {
	if _, err := os.Stat(rc.FilePath); err == nil {
		return true
	}
	return false
}

// NewFileRebootChecker is the constructor for the file based reboot checker
// TODO: Add extra input validation on filePath string here
func NewFileRebootChecker(filePath string) (*FileRebootChecker, error) {
	return &FileRebootChecker{
		FilePath: filePath,
	}, nil
}

// CommandChecker is using a custom command to check
// if a reboot is required. There are two modes of behaviour,
// if Privileged is granted, the NamespacePid is used to enter
// the given PID's namespace.
type CommandChecker struct {
	CheckCommand []string
	NamespacePid int
	Privileged   bool
}

// RebootRequired for CommandChecker runs a command without returning
// any eventual error. THis should be later refactored to remove the util wrapper
// and return the errors, instead of logging them here.
func (rc CommandChecker) RebootRequired() bool {
	var cmdline []string
	if rc.Privileged {
		cmdline = util.PrivilegedHostCommand(rc.NamespacePid, rc.CheckCommand)
	} else {
		cmdline = rc.CheckCommand
	}
	cmd := util.NewCommand(cmdline[0], cmdline[1:]...)
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

// NewCommandChecker is the constructor for the commandChecker, and by default
// runs new commands in a privileged fashion.
func NewCommandChecker(sentinelCommand string) (*CommandChecker, error) {
	cmd, err := shlex.Split(sentinelCommand)
	if err != nil {
		return nil, fmt.Errorf("error parsing provided sentinel command: %v", err)
	}
	return &CommandChecker{
		CheckCommand: cmd,
		NamespacePid: 1,
		Privileged:   true,
	}, nil
}
