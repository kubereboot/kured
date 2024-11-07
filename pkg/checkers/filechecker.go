package checkers

import "os"

// FileRebootChecker is the default reboot checker.
// It is unprivileged, and tests the presence of a files
type FileRebootChecker struct {
	FilePath string
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
