package reboot

import "context"

// Reboot interface defines the Reboot function to be implemented.
type Reboot interface {
	Reboot(context.Context)
}
