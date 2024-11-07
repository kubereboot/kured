// Package internal provides convenient tools which shouldn't be in the cmd/main
// It will eventually provide internal validation and chaining logic to select
// appropriate reboot and sentinel check methods based on configuration.
// It validates user input and instantiates the correct checker and rebooter implementations
// for use elsewhere in kured.
package internal
