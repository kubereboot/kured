package e2e

import (
	"encoding/json"
	"fmt"
	"os"

	jsonpatch "gopkg.in/evanphx/json-patch.v4"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	kuredDSManifestFilename string = "kured-ds.yaml"
)

var KuredDSSignalPatch = []map[string]interface{}{
	{"op": "replace", "path": "/spec/template/spec/containers/0/command", "value": []string{"/usr/bin/kured", "--reboot-sentinel=/sentinel/reboot-required", "--reboot-method=signal", "--period=6s"}},
	{"op": "replace", "path": "/spec/template/spec/containers/0/securityContext", "value": map[string]interface{}{
		"privileged":               false,
		"allowPrivilegeEscalation": false,
		"readOnlyRootFilesystem":   true,
		"capabilities": map[string]interface{}{
			"drop": []string{"*"},
			"add":  []string{"CAP_KILL"},
		},
	}},
}

var KuredDSCommandPatch = []map[string]interface{}{
	{"op": "replace", "path": "/spec/template/spec/containers/0/command", "value": []string{"/usr/bin/kured", "--reboot-sentinel=/sentinel/reboot-required", "--reboot-method=command", "--period=6s"}},
	{"op": "replace", "path": "/spec/template/spec/containers/0/securityContext", "value": map[string]interface{}{
		"privileged":             true,
		"readOnlyRootFilesystem": true,
	}},
}

// Constructor for DaemonSet with your provided defaults
func NewKuredDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "DaemonSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kured",
			Namespace: "kube-system",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "kured",
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "kured",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "kured",
					Tolerations: []corev1.Toleration{
						{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule},
						{Key: "node-role.kubernetes.io/master", Effect: corev1.TaintEffectNoSchedule},
					},
					HostPID:       true,
					RestartPolicy: corev1.RestartPolicyAlways,
					Volumes: []corev1.Volume{
						{
							Name: "sentinel",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/run",
									Type: func() *corev1.HostPathType {
										t := corev1.HostPathDirectory
										return &t
									}(),
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            "kured",
							Image:           kuredDevImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem: func() *bool { b := true; return &b }(),
							},
							Ports: []corev1.ContainerPort{
								{ContainerPort: 8080, Name: "metrics"},
							},
							Env: []corev1.EnvVar{
								{
									Name: "KURED_NODE_ID",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "spec.nodeName",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "sentinel",
									MountPath: "/sentinel",
									ReadOnly:  true,
								},
							},
							Command: []string{
								"/usr/bin/kured",
								"--reboot-sentinel=/sentinel/reboot-required",
								"--period=6s",
							},
						},
					},
				},
			},
		},
	}
}

// PatchDaemonSet mutates the ds by applying RFC6902 JSON Patch ops. Each patchOp is a map with "op", "path", "value".
func PatchDaemonSet(ds *appsv1.DaemonSet, patchOps ...[]map[string]interface{}) (*appsv1.DaemonSet, error) {
	orig, err := json.Marshal(ds)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	if len(patchOps) > 0 {
		patchBytes, err := json.Marshal(patchOps[0])
		if err != nil {
			return nil, fmt.Errorf("patch marshal: %w", err)
		}
		jp, err := jsonpatch.DecodePatch(patchBytes)
		if err != nil {
			return nil, fmt.Errorf("decode patch: %w", err)
		}
		patched, err := jp.Apply(orig)
		if err != nil {
			return nil, fmt.Errorf("apply patch: %w", err)
		}
		var dsPatched appsv1.DaemonSet
		if err := json.Unmarshal(patched, &dsPatched); err != nil {
			return nil, fmt.Errorf("unmarshal: %w", err)
		}
		if len(patchOps) == 1 {
			return &dsPatched, nil
		}
		return PatchDaemonSet(&dsPatched, patchOps[1:]...)
	}
	return ds, nil
}

// SaveDaemonsetToDisk is a convenience function allowing the reuse of Deploy Option
func SaveDaemonsetToDisk(ds *appsv1.DaemonSet, filename string) error {
	out, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal indent: %w", err)
	}
	return os.WriteFile(filename, out, 0644)
}
