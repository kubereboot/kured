package evacuators

// Evacuator is an interface used to implement business logic needed by some components before rebooting a node
type Evacuator interface {
	Evacuate() error
}
