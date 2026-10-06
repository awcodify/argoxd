package argocd

import (
	"context"
	"fmt"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
)

var _ LogStreamer = (*KubernetesSource)(nil)

// StreamLogs follows the logs of a Pod.
func (s *KubernetesSource) StreamLogs(ctx context.Context, _ string, pod explorer.ResourceNode) (<-chan LogEntry, error) {
	clients, err := s.clients()
	if err != nil {
		return nil, err
	}
	tail := int64(logTailLines)
	body, err := clients.typed.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{TailLines: &tail, Follow: true}).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("follow logs for %s: %w", pod.Name, err)
	}
	return streamLines(ctx, body, func(line []byte) (string, bool) { return string(line), true }), nil
}
