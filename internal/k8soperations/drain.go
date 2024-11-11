package k8soperations

import (
	"context"
	"fmt"
	"github.com/kubereboot/kured/internal/notifications"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	kubeDrain "k8s.io/kubectl/pkg/drain"
	"log/slog"
	"strconv"
	"time"
)

// Drain drains the node in a kured fashion, respecting delays, notifications, and applying labels/annotations.
func Drain(client *kubernetes.Clientset, node *v1.Node, preRebootNodeLabels []string, drainTimeout time.Duration, drainGracePeriod int, skipWaitForDeleteTimeoutSeconds int, drainPodSelector string, drainDelay time.Duration, messageTemplateDrain string, notifier notifications.Notifier) error {
	nodeName := node.GetName()

	if preRebootNodeLabels != nil {
		err := updateNodeLabels(client, node, preRebootNodeLabels)
		if err != nil {
			return fmt.Errorf("stopping drain due to problem with node labels %v", err)
		}
	}

	if drainDelay > 0 {
		slog.Debug("Delaying drain", "delay", drainDelay, "node", nodeName)
		time.Sleep(drainDelay)
	}

	slog.Info("Starting drain", "node", nodeName)

	notifier.Send(fmt.Sprintf(messageTemplateDrain, nodeName), "Starting drain")

	kubectlStdOutLogger := &slogWriter{message: "draining: results", stream: "stdout"}
	kubectlStdErrLogger := &slogWriter{message: "draining: results", stream: "stderr"}

	drainer := &kubeDrain.Helper{
		Client:                          client,
		Ctx:                             context.Background(),
		GracePeriodSeconds:              drainGracePeriod,
		PodSelector:                     drainPodSelector,
		SkipWaitForDeleteTimeoutSeconds: skipWaitForDeleteTimeoutSeconds,
		Force:                           true,
		DeleteEmptyDirData:              true,
		IgnoreAllDaemonSets:             true,
		ErrOut:                          kubectlStdErrLogger,
		Out:                             kubectlStdOutLogger,
		Timeout:                         drainTimeout,
	}

	// Add previous state of the node Spec.Unschedulable into an annotation
	// If an annotation was present, it means that either the cordon or drain failed,
	// hence it does not need to reapply: It might override what the user has set
	// (for example if the cordon succeeded but the drain failed)
	if _, ok := node.Annotations[KuredNodeWasUnschedulableBeforeDrainAnnotation]; !ok {
		// Store State of the node before cordon changes it
		annotations := map[string]string{KuredNodeWasUnschedulableBeforeDrainAnnotation: strconv.FormatBool(node.Spec.Unschedulable)}
		// & annotate this node with a timestamp so that other node maintenance tools know how long it's been since this node has been marked for reboot
		err := AddNodeAnnotations(client, nodeName, annotations)
		if err != nil {
			return fmt.Errorf("error saving state of the node %s, %v", nodeName, err)
		}
	}

	if err := kubeDrain.RunCordonOrUncordon(drainer, node, true); err != nil {
		return fmt.Errorf("error cordonning node %s, %v", nodeName, err)
	}

	if err := kubeDrain.RunNodeDrain(drainer, nodeName); err != nil {
		return fmt.Errorf("error draining node %s: %v", nodeName, err)
	}
	return nil
}

// Uncordon changes the `spec.Unschedulable` field of a node, applying kured labels and annotations.
// Is a noop on missing annotation or on a node in maintenance before kured action
func Uncordon(client *kubernetes.Clientset, node *v1.Node, notifier notifications.Notifier, postRebootNodeLabels []string, messageTemplateUncordon string) error {
	nodeName := node.GetName()
	// Revert cordon spec change with the help of node annotation
	annotationContent, ok := node.Annotations[KuredNodeWasUnschedulableBeforeDrainAnnotation]
	if !ok {
		// If no node annotations, uncordon will not act.
		// Do not uncordon if you do not know previous state, it could bring nodes under maintenance online!
		return nil
	}

	wasUnschedulable, err := strconv.ParseBool(annotationContent)
	if err != nil {
		return fmt.Errorf("annotation was edited and cannot be converted back to bool %v, cannot uncordon (unrecoverable)", err)
	}

	if wasUnschedulable {
		// Just delete the annotation, keep Cordonned
		err := DeleteNodeAnnotation(client, nodeName, KuredNodeWasUnschedulableBeforeDrainAnnotation)
		if err != nil {
			return fmt.Errorf("error removing the WasUnschedulable annotation, keeping the node stuck in cordonned state forever %v", err)
		}
		return nil
	}

	kubectlStdOutLogger := &slogWriter{message: "uncordon: results", stream: "stdout"}
	kubectlStdErrLogger := &slogWriter{message: "uncordon: results", stream: "stderr"}

	drainer := &kubeDrain.Helper{
		Client: client,
		ErrOut: kubectlStdErrLogger,
		Out:    kubectlStdOutLogger,
		Ctx:    context.Background(),
	}
	if err := kubeDrain.RunCordonOrUncordon(drainer, node, false); err != nil {
		return fmt.Errorf("error uncordonning node %s: %v", nodeName, err)
	} else if postRebootNodeLabels != nil {
		err := updateNodeLabels(client, node, postRebootNodeLabels)
		return fmt.Errorf("error updating node (%s) labels, needs manual intervention %v", nodeName, err)
	}

	err = DeleteNodeAnnotation(client, nodeName, KuredNodeWasUnschedulableBeforeDrainAnnotation)
	if err != nil {
		return fmt.Errorf("error removing the WasUnschedulable annotation, keeping the node stuck in current state forever %v", err)
	}
	notifier.Send(fmt.Sprintf(messageTemplateUncordon, nodeName), "Node uncordonned successfully")
	return nil
}
