package main

import (
	"fmt"
	"os"

	"github.com/google/shlex"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type osSpecificBehaviors interface {
	getClusterCofig() (*rest.Config, error)
	getSentinelCommand(rebootSentinelFile, rebootSentinelCommand string) []string
	getRebootCommand(rebootCommand string) []string
}

type linuxHelper struct {
	// PID on host to nsenter into in order to run commands on the host.
	pid int
}

func (linuxHelper) getClusterCofig() (*rest.Config, error) {
	return rest.InClusterConfig()
}

func (l linuxHelper) getSentinelCommand(rebootSentinelFile, rebootSentinelCommand string) []string {
	return buildHostCommand(l.pid, l.buildSentinelCommand(rebootSentinelFile, rebootSentinelCommand))
}

func (linuxHelper) buildSentinelCommand(rebootSentinelFile, rebootSentinelCommand string) []string {
	if rebootSentinelCommand != "" {
		cmd, err := shlex.Split(rebootSentinelCommand)
		if err != nil {
			log.Fatalf("Error parsing provided sentinel command: %v", err)
		}
		return cmd
	}
	return []string{"test", "-f", rebootSentinelFile}
}

func (l linuxHelper) getRebootCommand(rebootCommand string) []string {
	return buildHostCommand(l.pid, parseRebootCommand(rebootCommand))
}

// parseRebootCommand creates the shell command line which will need wrapping to escape
// the container boundaries
func parseRebootCommand(rebootCommand string) []string {

	command, err := shlex.Split(rebootCommand)
	if err != nil {
		log.Fatalf("Error parsing provided reboot command: %v", err)
	}
	return command
}

// To run those commands as it was the host, we'll use nsenter
// Relies on hostPID:true and privileged:true to enter host mount space
func buildHostCommand(pid int, command []string) []string {

	// From the container, we nsenter into the proper PID to run the hostCommand.
	// For this, kured daemonset need to be configured with hostPID:true and privileged:true
	cmd := []string{"/usr/bin/nsenter", fmt.Sprintf("-m/proc/%d/ns/mnt", pid), "--"}
	cmd = append(cmd, command...)
	return cmd
}

type windowsHelper struct {
}

func (windowsHelper) getClusterCofig() (*rest.Config, error) {
	// Note: InClusterConfig() does not currently work for host process containers.
	// See https://github.com/kubernetes/kubernetes/pull/104490
	// Instead Kured-Init.ps1 creates kubeconfig.conf file which uses
	// the ca.crt / token file for the current service account and we will load that here.
	var kubeConfigPath = os.ExpandEnv("${CONTAINER_SANDBOX_MOUNT_POINT}\\var\\run\\secrets\\kubernetes.io\\serviceaccount\\kubeconfig.conf")
	return clientcmd.BuildConfigFromFlags("", kubeConfigPath)
}

func (windowsHelper) getSentinelCommand(rebootSentinelFile, rebootSentinelCommand string) []string {
	// On Windows always run Test-PendingReboot.ps1 to detect if a reboot is required
	return []string{"powershell.exe", "./Test-PendingReboot.ps1"}
}

func (windowsHelper) getRebootCommand(rebootCommand string) []string {
	// Issue a reboot with shutdown.exe.
	// - /f Force shutdown without prompting users
	// - /r reboot
	// - /t 5 to delay shutdown by 5 seconds to allow logging to get captured
	// - /c kured specify kured initiated the reboot
	return []string{"shutdown.exe", "/f", "/r", "/t", "5", "/c", "kured"}
}
