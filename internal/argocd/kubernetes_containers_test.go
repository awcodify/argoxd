package argocd

import (
	"context"
	"slices"
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// containerPod is a Pod of the web workload with the given containers.
func containerPod(name string, annotations map[string]string, containers ...string) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Namespace: "store", Name: name, Labels: map[string]string{"app": "web"}, Annotations: annotations,
	}}
	for _, container := range containers {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: container})
	}
	return pod
}

func containerNode(name string) explorer.ResourceNode {
	return explorer.ResourceNode{Version: "v1", Kind: "Pod", Namespace: "store", Name: name}
}

// streamedContainers collects "pod/container" for every line of a stream.
func streamedContainers(t *testing.T, stream <-chan LogEntry) []string {
	t.Helper()
	var seen []string
	for _, entry := range collect(t, stream) {
		if entry.Err != nil {
			t.Fatalf("stream error: %v", entry.Err)
		}
		seen = append(seen, entry.Pod+"/"+entry.Container)
	}
	slices.Sort(seen)
	return seen
}

func TestKubernetesSourceStreamsAPodsDefaultContainer(t *testing.T) {
	ctx := context.Background()
	plain := workloadSource(selectorDeployment(), containerPod("web-abc", nil, "app", "sidecar"))
	annotated := workloadSource(selectorDeployment(),
		containerPod("web-abc", map[string]string{defaultContainerAnnotation: "sidecar"}, "app", "sidecar"))

	first, err := plain.StreamLogs(ctx, "checkout", containerNode("web-abc"), "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := annotated.StreamLogs(ctx, "checkout", containerNode("web-abc"), "")
	if err != nil {
		t.Fatal(err)
	}

	if got := streamedContainers(t, first); !slices.Equal(got, []string{"web-abc/app"}) {
		t.Fatalf("plain pod streamed %v, want its first container", got)
	}
	if got := streamedContainers(t, second); !slices.Equal(got, []string{"web-abc/sidecar"}) {
		t.Fatalf("annotated pod streamed %v, want the annotated container", got)
	}
}

func TestKubernetesSourceStreamsTheChosenOrEveryContainerOfAPod(t *testing.T) {
	source := workloadSource(selectorDeployment(), containerPod("web-abc", nil, "app", "sidecar"))

	chosen, err := source.StreamLogs(context.Background(), "checkout", containerNode("web-abc"), "sidecar")
	if err != nil {
		t.Fatal(err)
	}
	every, err := source.StreamLogs(context.Background(), "checkout", containerNode("web-abc"), AllContainers)
	if err != nil {
		t.Fatal(err)
	}

	if got := streamedContainers(t, chosen); !slices.Equal(got, []string{"web-abc/sidecar"}) {
		t.Fatalf("chosen container streamed %v", got)
	}
	if got := streamedContainers(t, every); !slices.Equal(got, []string{"web-abc/app", "web-abc/sidecar"}) {
		t.Fatalf("all containers streamed %v", got)
	}
}

func TestKubernetesSourceStreamsAWorkloadsContainersPerPod(t *testing.T) {
	source := workloadSource(selectorDeployment(),
		containerPod("web-abc", nil, "app", "sidecar"),
		containerPod("web-def", map[string]string{defaultContainerAnnotation: "sidecar"}, "app", "sidecar"))

	defaults, err := source.StreamLogs(context.Background(), "checkout", webDeployment, "")
	if err != nil {
		t.Fatal(err)
	}
	every, err := source.StreamLogs(context.Background(), "checkout", webDeployment, AllContainers)
	if err != nil {
		t.Fatal(err)
	}

	if got := streamedContainers(t, defaults); !slices.Equal(got, []string{"web-abc/app", "web-def/sidecar"}) {
		t.Fatalf("defaults streamed %v, want each pod's own default container", got)
	}
	if got := streamedContainers(t, every); !slices.Equal(got, []string{"web-abc/app", "web-abc/sidecar", "web-def/app", "web-def/sidecar"}) {
		t.Fatalf("all containers streamed %v", got)
	}
}

func TestKubernetesSourceSnapshotOfAChosenOrEveryContainer(t *testing.T) {
	source := workloadSource(selectorDeployment(), containerPod("web-abc", nil, "app", "sidecar"))
	pod := containerNode("web-abc")

	one, err := source.ResourceLogs(context.Background(), "checkout", pod, "sidecar")
	if err != nil || one != "fake logs" {
		t.Fatalf("ResourceLogs(sidecar) = %q, %v", one, err)
	}

	all, err := source.ResourceLogs(context.Background(), "checkout", pod, AllContainers)
	if err != nil || all != "app │ fake logs\nsidecar │ fake logs" {
		t.Fatalf("ResourceLogs(all) = %q, %v, want each container's lines under its name", all, err)
	}
}
