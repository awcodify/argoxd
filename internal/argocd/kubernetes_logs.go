package argocd

import (
	"context"
	"fmt"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

var _ LogStreamer = (*KubernetesSource)(nil)

// StreamLogs follows the logs of a Pod, or of the Pods a workload selects. An
// empty container follows each Pod's default container.
func (s *KubernetesSource) StreamLogs(ctx context.Context, _ string, resource explorer.ResourceNode, container string) (<-chan LogEntry, error) {
	clients, err := s.clients()
	if err != nil {
		return nil, err
	}
	if resource.Kind == "Pod" {
		pod := readPod(ctx, clients.typed, resource.Namespace, resource.Name, container)
		return streamContainers(ctx, clients.typed, resource.Namespace, resource.Name, containersToRead(pod, container))
	}

	pods, err := s.workloadPods(ctx, clients, resource)
	if err != nil {
		return nil, err
	}
	followed := pods[:min(len(pods), maxFollowedPods)]
	streams := make([]<-chan LogEntry, 0, len(followed)+1)
	for index := range followed {
		pod := &followed[index]
		stream, err := streamContainers(ctx, clients.typed, resource.Namespace, pod.Name, containersToRead(pod, container))
		if err != nil {
			stream = singleEntry(LogEntry{Pod: pod.Name, Line: "cannot read logs: " + err.Error()})
		}
		streams = append(streams, stream)
	}
	if len(pods) > len(followed) {
		streams = append(streams, singleEntry(LogEntry{Line: fmt.Sprintf("following the first %d of %d pods", len(followed), len(pods))}))
	}
	return mergeStreams(ctx, streams...), nil
}

// readPod fetches a Pod to learn its containers, which is only needed when
// none is chosen. A Pod that cannot be read is treated as declaring none, which
// leaves the choice of container to the API server.
func readPod(ctx context.Context, client kubernetes.Interface, namespace, name, container string) *corev1.Pod {
	if container != "" && container != AllContainers {
		return &corev1.Pod{}
	}
	pod, err := client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return &corev1.Pod{}
	}
	return pod
}

// containersToRead names the containers to read for a Pod. An empty name leaves
// the choice to the API server, which refuses when a Pod has several.
func containersToRead(pod *corev1.Pod, container string) []string {
	if container != "" && container != AllContainers {
		return []string{container}
	}
	names := make([]string, 0, len(pod.Spec.Containers))
	for _, declared := range pod.Spec.Containers {
		names = append(names, declared.Name)
	}
	switch {
	case len(names) == 0:
		return []string{""}
	case container == AllContainers:
		return names
	}
	return []string{defaultContainer(names, pod.Annotations)}
}

// streamContainers follows the given containers of one Pod as a single stream.
func streamContainers(ctx context.Context, client kubernetes.Interface, namespace, pod string, containers []string) (stream <-chan LogEntry, err error) {
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		if err != nil {
			cancel()
		}
	}()

	streams := make([]<-chan LogEntry, 0, len(containers))
	for _, container := range containers {
		opened, err := streamContainer(ctx, client, namespace, pod, container)
		if err != nil {
			return nil, err
		}
		streams = append(streams, opened)
	}
	if len(streams) == 1 {
		return streams[0], nil
	}
	return mergeStreams(ctx, streams...), nil
}

func streamContainer(ctx context.Context, client kubernetes.Interface, namespace, pod, container string) (<-chan LogEntry, error) {
	tail := int64(logTailLines)
	options := &corev1.PodLogOptions{Container: container, TailLines: &tail, Follow: true}
	body, err := client.CoreV1().Pods(namespace).GetLogs(pod, options).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("follow logs for %s: %w", pod, err)
	}
	return streamLines(ctx, body, func(line []byte) (LogEntry, bool) {
		return LogEntry{Pod: pod, Container: container, Line: string(line)}, true
	}), nil
}

// workloadPods returns the Pods a workload's selector matches, in name order.
func (s *KubernetesSource) workloadPods(ctx context.Context, clients kubernetesClients, workload explorer.ResourceNode) ([]corev1.Pod, error) {
	plural, _ := meta.UnsafeGuessKindToResource(schema.GroupVersionKind{Group: workload.Group, Version: workload.Version, Kind: workload.Kind})
	object, err := clients.dynamic.Resource(plural).Namespace(workload.Namespace).Get(ctx, workload.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s/%s: %w", workload.Kind, workload.Name, err)
	}

	fields, found, _ := unstructured.NestedMap(object.Object, "spec", "selector")
	if !found {
		return nil, fmt.Errorf("%s/%s has no selector to find its pods", workload.Kind, workload.Name)
	}
	var labelSelector metav1.LabelSelector
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(fields, &labelSelector); err != nil {
		return nil, fmt.Errorf("read the selector of %s/%s: %w", workload.Kind, workload.Name, err)
	}
	selector, err := metav1.LabelSelectorAsSelector(&labelSelector)
	if err != nil || selector.Empty() {
		return nil, fmt.Errorf("%s/%s has no usable selector to find its pods", workload.Kind, workload.Name)
	}

	list, err := clients.typed.CoreV1().Pods(workload.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return nil, fmt.Errorf("list the pods of %s/%s: %w", workload.Kind, workload.Name, err)
	}
	if len(list.Items) == 0 {
		return nil, fmt.Errorf("%s/%s has no pods", workload.Kind, workload.Name)
	}
	return list.Items, nil
}
