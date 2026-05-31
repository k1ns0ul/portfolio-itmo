package k8s

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NodeJobSpec struct {
	SessionID       string
	NodeID          int
	Image           string
	Namespace       string
	CoordinatorAddr string
}

func (s NodeJobSpec) jobName() string {
	return fmt.Sprintf("mpc-node-%s-%d", s.SessionID, s.NodeID)
}

func buildJob(s NodeJobSpec) *batchv1.Job {
	labels := map[string]string{
		"app":     "mpc-node",
		"session": s.SessionID,
		"node-id": fmt.Sprintf("%d", s.NodeID),
	}
	backoff := int32(0)
	ttl := int32(600)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.jobName(),
			Namespace: s.Namespace,
			Labels:    labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoff,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "node",
							Image: s.Image,
							Ports: []corev1.ContainerPort{{ContainerPort: 9091, Name: "peer"}},
							Env: []corev1.EnvVar{
								{Name: "NODE_ID", Value: fmt.Sprintf("%d", s.NodeID)},
								{Name: "SESSION_ID", Value: s.SessionID},
								{Name: "COORDINATOR_ADDR", Value: s.CoordinatorAddr},
								{Name: "LISTEN_ADDR", Value: ":9091"},
								{
									Name: "POD_IP",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("256Mi"),
									corev1.ResourceCPU:    resource.MustParse("250m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("256Mi"),
									corev1.ResourceCPU:    resource.MustParse("500m"),
								},
							},
						},
					},
				},
			},
		},
	}
}
