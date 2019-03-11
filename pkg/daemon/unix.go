package daemon

import (
	"regexp"
	"time"
)

type UnixDaemon struct {
	commonDaemon *CommonDaemon
}

func NewUnixDaemon(period time.Duration, dsNamespace string, dsName string, lockAnnotation string, prometheusURL string, alertFilter *regexp.Regexp, rebootSentinel string, slackHookURL string, slackUsername string, podSelectors []string) *unix {
	c := core{period, dsNamespace, dsName, lockAnnotation, prometheusURL, alertFilter, rebootSentinel, slackHookURL, slackUsername, podSelectors}
	return &UnixDaemon{&c}
}

func (ud UnixDaemon) drain(nodeID string) {
	commonDaemon.drain("/usr/bin/kubectl", "drain", "--ignore-daemonsets", "--delete-local-data", "--force", nodeID)
}

func (ud UnixDaemon) uncordon(nodeID string) {
	commonDaemon.uncordon(nodeID, "/usr/bin/kubectl", "uncordon", nodeID)
}

func (ud UnixDaemon) commandReboot(nodeID string) {
	commonDaemon.commandReboot(nodeID, "/usr/bin/nsenter", "-m/proc/1/ns/mnt", "/bin/systemctl", "reboot")
}

func (ud UnixDaemon) sentinelExists() bool {
	// Relies on hostPID:true and privileged:true to enter host mount space
	return commonDaemon.sentinelExists("/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "/usr/bin/test", "-f")
}

func (ud UnixDaemon) rebootAsRequired(nodeID string) {
	commonDaemon.rebootAsRequired(nodeID)
}

func (ud UnixDaemon) maintainRebootRequiredMetric(nodeID string) {
	commonDaemon.maintainRebootRequiredMetric(nodeID)
}
