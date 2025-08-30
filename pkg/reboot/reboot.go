// Package reboot provides mechanisms to trigger node reboots using different
// methods, like custom commands or signals.
// Each of those includes constructors and interfaces for handling different reboot
// strategies, supporting privileged command execution via nsenter for containerized environments.
package reboot

// Rebooter is the standard interface to use to execute
// the reboot, after it has been considered as necessary.
// The Reboot method does not expect any return, yet should
// most likely be refactored in the future to return an error
type Rebooter interface {
	Reboot() error
}
