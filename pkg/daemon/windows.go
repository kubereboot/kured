package daemon

import (
	"regexp"
	"time"
)

type WindowsDaemon struct {
	commonDaemon *CommonDaemon
	kubeCtlPath  string
}

func NewWindowsDaemon(kubeCtlPath string, period time.Duration, dsNamespace string, dsName string, lockAnnotation string, prometheusURL string, alertFilter *regexp.Regexp, rebootSentinel string, slackHookURL string, slackUsername string, podSelectors []string) *windows {
	c := core{period, dsNamespace, dsName, lockAnnotation, prometheusURL, alertFilter, rebootSentinel, slackHookURL, slackUsername, podSelectors}
	return &WindowsDaemon{&c, kubeCtlPath}
}

func (wd WindowsDaemon) drain(nodeID string) {
	commonDaemon.drain(kubeCtlPath, "drain", "--ignore-daemonsets", "--delete-local-data", "--force", nodeID)
}

func (wd WindowsDaemon) uncordon(nodeID string) {
	commonDaemon.uncordon(nodeID, kubeCtlPath, "uncordon", nodeID)
}

func (wd WindowsDaemon) commandReboot(nodeID string) {
	commonDaemon.commandReboot(nodeID, "shutdown", "/g", "/d", "/f")
}

func (wd WindowsDaemon) sentinelExists() bool {
	return commonDaemon.sentinelExists("type")
}

func (wd WindowsDaemon) rebootAsRequired(nodeID string) {
	commonDaemon.rebootAsRequired(nodeID)
}

func (wd WindowsDaemon) maintainRebootRequiredMetric(nodeID string) {
	commonDaemon.maintainRebootRequiredMetric(nodeID)
}
