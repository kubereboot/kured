// Package blockers provides interfaces and implementations for determining
// whether a system should be prevented to reboot.
// You can use that package if you fork Kured's main loop.
package blockers

// RebootBlocked checks that a single block Checker
// will block the reboot or not.
func RebootBlocked(blockers ...RebootBlocker) bool {
	for _, blocker := range blockers {
		if blocker.IsBlocked() {
			return true
		}
	}
	return false
}

// RebootBlocker interface should be implemented by types
// to know if their instantiations should block a reboot
type RebootBlocker interface {
	IsBlocked() bool
}
