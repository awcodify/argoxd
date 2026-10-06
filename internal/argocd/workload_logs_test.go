package argocd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/awcodify/argoxd/internal/explorer"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	typedfake "k8s.io/client-go/kubernetes/fake"
)

var webDeployment = explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web"}

// collect reads a stream until it closes.
func collect(t *testing.T, stream <-chan LogEntry) []LogEntry {
	t.Helper()
	var entries []LogEntry
	for {
		select {
		case entry, open := <-stream:
			if !open {
				return entries
			}
			entries = append(entries, entry)
		case <-time.After(3 * time.Second):
			t.Fatalf("the stream did not close; got %+v", entries)
		}
	}
}

func TestAPISourceStreamsAWorkloadsLogsWithTheirPods(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("kind") != "Deployment" || query.Get("group") != "apps" || query.Get("resourceName") != "web" ||
			query.Get("namespace") != "store" || query.Get("follow") != "true" || query.Has("podName") {
			t.Errorf("logs query = %v, want the Deployment and no pod", query)
		}
		_, _ = w.Write([]byte(`{"result":{"content":"hello","podName":"web-abc"}}` + "\n" +
			`{"result":{"content":"world","podName":"web-def"}}` + "\n" +
			`{"result":{"content":"","last":true,"podName":""}}` + "\n"))
	}))
	defer server.Close()

	stream, err := NewAPISource(server.URL, "", false).StreamLogs(context.Background(), "checkout", webDeployment, "web")
	if err != nil {
		t.Fatal(err)
	}

	entries := collect(t, stream)
	if len(entries) != 2 || entries[0] != (LogEntry{Pod: "web-abc", Container: "web", Line: "hello"}) || entries[1] != (LogEntry{Pod: "web-def", Container: "web", Line: "world"}) {
		t.Fatalf("entries = %+v, want two lines tagged with their pods", entries)
	}
}

func TestMergeStreamsInterleavesAndClosesWhenAllEnd(t *testing.T) {
	first, second := make(chan LogEntry, 2), make(chan LogEntry, 2)
	first <- LogEntry{Pod: "a", Line: "a1"}
	first <- LogEntry{Pod: "a", Err: errors.New("boom")}
	second <- LogEntry{Pod: "b", Line: "b1"}
	close(first)
	close(second)

	entries := collect(t, mergeStreams(context.Background(), first, second))

	lines := map[string]string{}
	for _, entry := range entries {
		if entry.Err != nil {
			t.Fatalf("a Pod's failure ended the merged stream: %+v", entry)
		}
		lines[entry.Pod] += entry.Line + ";"
	}
	if len(entries) != 3 || !strings.HasPrefix(lines["a"], "a1;") || !strings.Contains(lines["a"], "boom") || lines["b"] != "b1;" {
		t.Fatalf("entries = %+v, want both Pods' lines and the failure as a line of Pod a", entries)
	}
}

func TestMergeStreamsClosesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	merged := mergeStreams(ctx, make(chan LogEntry), make(chan LogEntry))

	cancel()

	expectClosed(t, merged)
}

func workloadPod(name string, labels map[string]string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "store", Name: name, Labels: labels}}
}

func selectorDeployment() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]any{"namespace": "store", "name": "web"},
		"spec": map[string]any{"selector": map[string]any{
			"matchLabels": map[string]any{"app": "web"},
		}},
	}}
}

func workloadSource(workload *unstructured.Unstructured, pods ...runtime.Object) *KubernetesSource {
	client := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "apps", Version: "v1", Resource: "deployments"}: "DeploymentList",
	}, workload)
	return &KubernetesSource{
		namespace: "argocd",
		clients: func() (kubernetesClients, error) {
			return kubernetesClients{dynamic: client, typed: typedfake.NewClientset(pods...)}, nil
		},
	}
}

func TestKubernetesSourceStreamsTheLogsOfEveryPodOfAWorkload(t *testing.T) {
	source := workloadSource(selectorDeployment(),
		workloadPod("web-abc", map[string]string{"app": "web"}),
		workloadPod("web-def", map[string]string{"app": "web"}),
		workloadPod("other-xyz", map[string]string{"app": "other"}))

	stream, err := source.StreamLogs(context.Background(), "checkout", webDeployment, "")
	if err != nil {
		t.Fatal(err)
	}

	pods := map[string]string{}
	for _, entry := range collect(t, stream) {
		pods[entry.Pod] = entry.Line
	}
	if len(pods) != 2 || pods["web-abc"] != "fake logs" || pods["web-def"] != "fake logs" {
		t.Fatalf("pods = %v, want the logs of web-abc and web-def only", pods)
	}
}

func TestKubernetesSourceStreamTagsASinglePodsLines(t *testing.T) {
	stream, err := fakeKubernetesSource().StreamLogs(context.Background(), "checkout", webPod, "")
	if err != nil {
		t.Fatal(err)
	}

	if entry := receive(t, stream); entry.Pod != "web-1-abc" {
		t.Fatalf("entry = %+v, want the Pod's name", entry)
	}
}

func TestKubernetesSourceWorkloadWithoutPodsIsAnError(t *testing.T) {
	source := workloadSource(selectorDeployment(), workloadPod("other-xyz", map[string]string{"app": "other"}))

	if _, err := source.StreamLogs(context.Background(), "checkout", webDeployment, ""); err == nil || !strings.Contains(err.Error(), "no pods") {
		t.Fatalf("error = %v, want a message that the workload has no pods", err)
	}
}

func TestKubernetesSourceWorkloadWithoutASelectorIsAnError(t *testing.T) {
	workload := selectorDeployment()
	unstructured.RemoveNestedField(workload.Object, "spec", "selector")

	if _, err := workloadSource(workload).StreamLogs(context.Background(), "checkout", webDeployment, ""); err == nil || !strings.Contains(err.Error(), "selector") {
		t.Fatalf("error = %v, want a message about the missing selector", err)
	}
}

func TestKubernetesSourceFollowsAtMostMaxFollowedPods(t *testing.T) {
	var pods []runtime.Object
	for index := range maxFollowedPods + 2 {
		pods = append(pods, workloadPod(fmt.Sprintf("web-%02d", index), map[string]string{"app": "web"}))
	}

	stream, err := workloadSource(selectorDeployment(), pods...).StreamLogs(context.Background(), "checkout", webDeployment, "")
	if err != nil {
		t.Fatal(err)
	}

	followed, notice := map[string]bool{}, ""
	for _, entry := range collect(t, stream) {
		if entry.Pod == "" {
			notice = entry.Line
			continue
		}
		followed[entry.Pod] = true
	}
	if len(followed) != maxFollowedPods || !strings.Contains(notice, "30 of 32") {
		t.Fatalf("followed %d pods with notice %q, want %d and a notice naming 30 of 32", len(followed), notice, maxFollowedPods)
	}
}

func TestDemoSourceInterleavesTheLogsOfAWorkloadsPods(t *testing.T) {
	source := NewDemoSource()
	source.logInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deployment := explorer.ResourceNode{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "cart"}

	stream, err := source.StreamLogs(ctx, "cart", deployment, "")
	if err != nil {
		t.Fatal(err)
	}

	pods := map[string]bool{}
	for range 30 {
		entry := receive(t, stream)
		if entry.Pod == "" || !strings.HasPrefix(entry.Pod, "cart-") {
			t.Fatalf("entry = %+v, want a line tagged with one of the Deployment's Pods", entry)
		}
		pods[entry.Pod] = true
	}
	if len(pods) != 3 {
		t.Fatalf("saw %d pods, want all 3 of the Deployment", len(pods))
	}
}
