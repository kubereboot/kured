// Package blockers provides interfaces and implementations for determining
// whether a system should be prevented to reboot.
// You can use that package if you fork Kured's main loop.
package blockers

// RebootBlocked returns whether at least one blocker
// amongst all the blockers IS CURRENTLY blocking the reboot
// and also returns the subset of those blockers CURRENTLY BLOCKING
// the reboot.
func RebootBlocked(blockers ...RebootBlocker) (blocked bool, blocking []RebootBlocker) {
	for _, blocker := range blockers {
		if blocker.IsBlocked() {
			blocked = true
			blocking = append(blocking, blocker)
		}
	}
	return
}

// RebootBlocker interface should be implemented by types
// to know if their instantiations should block a reboot
type RebootBlocker interface {
	IsBlocked() bool
	MetricLabel() string
}
