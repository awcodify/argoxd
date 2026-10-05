package argocd

import (
	"testing"

	"github.com/awcodify/argoxd/internal/explorer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAttachOwnedResourcesFollowsOwnerReferences(t *testing.T) {
	tree := explorer.ResourceTree{Nodes: []explorer.ResourceNode{
		{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "store", Name: "web"},
	}}
	pod := object("v1", "Pod", "store", "web-1-abc", owner("apps/v1", "ReplicaSet", "web-1"))
	pod.Object["status"] = map[string]any{"phase": "Pending"}
	replicaSet := object("apps/v1", "ReplicaSet", "store", "web-1", owner("apps/v1", "Deployment", "web"))
	unrelated := object("v1", "Pod", "store", "other-1-xyz", owner("apps/v1", "ReplicaSet", "other-1"))

	got := attachOwnedResources(tree, []unstructured.Unstructured{pod, replicaSet, unrelated})

	if len(got.Nodes) != 3 {
		t.Fatalf("nodes = %+v, want deployment, replica set and pod", got.Nodes)
	}
	hierarchy := got.Hierarchy()
	if len(hierarchy) != 1 || len(hierarchy[0].Children) != 1 || len(hierarchy[0].Children[0].Children) != 1 {
		t.Fatalf("hierarchy = %+v, want Deployment → ReplicaSet → Pod", hierarchy)
	}
	attachedPod := hierarchy[0].Children[0].Children[0]
	if attachedPod.Name != "web-1-abc" || attachedPod.Version != "v1" || attachedPod.Health != "Progressing" {
		t.Fatalf("pod = %+v, want progressing web-1-abc", attachedPod)
	}
}

func TestLiveHealth(t *testing.T) {
	tests := map[string]struct {
		object unstructured.Unstructured
		want   string
	}{
		"ready pod": {withStatus("v1", "Pod", map[string]any{
			"phase":             "Running",
			"containerStatuses": []any{map[string]any{"ready": true}},
		}), "Healthy"},
		"crash looping pod": {withStatus("v1", "Pod", map[string]any{
			"phase": "Running",
			"containerStatuses": []any{map[string]any{
				"ready": false,
				"state": map[string]any{"waiting": map[string]any{"reason": "CrashLoopBackOff"}},
			}},
		}), "Degraded"},
		"failed pod":         {withStatus("v1", "Pod", map[string]any{"phase": "Failed"}), "Degraded"},
		"completed pod":      {withStatus("v1", "Pod", map[string]any{"phase": "Succeeded"}), "Healthy"},
		"scaling replicaset": {withStatus("apps/v1", "ReplicaSet", map[string]any{"replicas": int64(2), "readyReplicas": int64(1)}), "Progressing"},
		"ready replicaset":   {withStatus("apps/v1", "ReplicaSet", map[string]any{"replicas": int64(2), "readyReplicas": int64(2)}), "Healthy"},
		"succeeded job":      {withStatus("batch/v1", "Job", map[string]any{"succeeded": int64(1)}), "Healthy"},
		"failed job": {withStatus("batch/v1", "Job", map[string]any{
			"conditions": []any{map[string]any{"type": "Failed", "status": "True"}},
		}), "Degraded"},
		"running job": {withStatus("batch/v1", "Job", map[string]any{"active": int64(1)}), "Progressing"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if got := liveHealth(test.object); got != test.want {
				t.Fatalf("liveHealth() = %q, want %q", got, test.want)
			}
		})
	}
}

func object(apiVersion, kind, namespace, name string, owners ...any) unstructured.Unstructured {
	return unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"namespace": namespace, "name": name, "ownerReferences": owners},
	}}
}

func owner(apiVersion, kind, name string) map[string]any {
	return map[string]any{"apiVersion": apiVersion, "kind": kind, "name": name}
}

func withStatus(apiVersion, kind string, status map[string]any) unstructured.Unstructured {
	resource := object(apiVersion, kind, "default", "example")
	resource.Object["status"] = status
	return resource
}
