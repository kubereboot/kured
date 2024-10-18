package blockers

// RebootBlocker interface should be implemented by types
// to know if their instantiations should block a reboot
// As blockers are now exported as labels, the blocker must
// implement a MetricLabel method giving a label for the
// blocker, with low cardinality if possible.
type RebootBlocker interface {
	IsBlocked() bool
	MetricLabel() string
}
