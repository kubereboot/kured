package telemetrics

import (
        "os"
        "os/exec"
	"fmt"
	"github.com/sirupsen/logrus"
	"strconv"
)

func notify(severity int, message string) error {
        // Relies on hostPID:true and privileged:true to enter host mount space
        telemCmd := exec.Command("/usr/bin/nsenter", "-m/proc/1/ns/mnt",
        "--", "/usr/bin/telem-record-gen", "-s", strconv.Itoa(severity), "-c",
        "works.weave/kured/reboot", "--payload", message)
        if err := telemCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error invoking telm-record-gen command: %v", err)
		return err
        }
	return nil
}

func NotifyDrain(nodeID string) error {
	return notify(1, fmt.Sprintf("Draining node %s", nodeID))
}

func NotifyReboot(nodeID string) error {
	return notify(1, fmt.Sprintf("Rebooting node %s", nodeID))
}

type TelemetricsHook struct {
        enabled bool
	min_level int
}

func NewTelemetricsHook(enabled bool, min_level int) (*TelemetricsHook, error) {
	return &TelemetricsHook{enabled, min_level}, nil
}

func (hook *TelemetricsHook) Fire(entry *logrus.Entry) error {
	switch entry.Level {
	case logrus.PanicLevel, logrus.FatalLevel:
		if hook.min_level <= 4 {
			return notify (4, entry.Message)
		}
	case logrus.ErrorLevel:
		if hook.min_level <= 3 {
			return notify (3, entry.Message)
		}
	case logrus.WarnLevel:
		if hook.min_level <= 2 {
			return notify (2, entry.Message)
		}
	case logrus.InfoLevel:
		if hook.min_level <= 1 {
			return notify (1, entry.Message)
		}
	case logrus.DebugLevel, logrus.TraceLevel:
		if hook.min_level == 0 {
			return notify (1, entry.Message)
		}
	default:
		return nil
	}
	return nil
}

func (hook *TelemetricsHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
