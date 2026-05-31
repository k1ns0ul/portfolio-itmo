package k8s

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	cs *kubernetes.Clientset
}

func NewInCluster() (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("load in-cluster config: %w", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build clientset: %w", err)
	}
	return &Client{cs: cs}, nil
}

func (c *Client) CreateNodeJob(ctx context.Context, spec NodeJobSpec) error {
	job := buildJob(spec)
	_, err := c.cs.BatchV1().Jobs(spec.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("create job %s: %w", spec.jobName(), err)
	}
	return nil
}

func (c *Client) DeleteSessionJobs(ctx context.Context, namespace, sessionID string) error {
	policy := metav1.DeletePropagationBackground
	selector := "session=" + sessionID
	err := c.cs.BatchV1().Jobs(namespace).DeleteCollection(ctx,
		metav1.DeleteOptions{PropagationPolicy: &policy},
		metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("delete jobs for session %s: %w", sessionID, err)
	}
	return nil
}

func (c *Client) GetNodePodIP(ctx context.Context, namespace, sessionID string, nodeID int) (string, error) {
	selector := fmt.Sprintf("session=%s,node-id=%d", sessionID, nodeID)
	deadline := time.Now().Add(60 * time.Second)
	for {
		pods, err := c.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return "", fmt.Errorf("list pods for node %d: %w", nodeID, err)
		}
		for _, p := range pods.Items {
			if p.Status.PodIP != "" {
				return p.Status.PodIP, nil
			}
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("pod for node %d did not get an IP in time", nodeID)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func (c *Client) WatchJobStatus(ctx context.Context, namespace, sessionID string) (map[string]bool, error) {
	selector := "session=" + sessionID
	jobs, err := c.cs.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list jobs for %s: %w", sessionID, err)
	}
	out := make(map[string]bool, len(jobs.Items))
	for _, j := range jobs.Items {
		out[j.Name] = j.Status.Succeeded > 0
	}
	return out, nil
}
