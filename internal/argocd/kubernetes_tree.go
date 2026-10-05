package argocd

import (
	"slices"

	"github.com/awcodify/argoxd/internal/explorer"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ownedResources are created by Kubernetes on behalf of managed workloads.
// Argo CD leaves them out of the Application status, so they are looked up live.
var ownedResources = []schema.GroupVersionResource{
	{Group: "apps", Version: "v1", Resource: "replicasets"},
	{Group: "batch", Version: "v1", Resource: "jobs"},
	{Version: "v1", Resource: "pods"},
}

// failingWaitReasons mark a container that will not start without intervention.
var failingWaitReasons = []string{"CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError"}

// attachOwnedResources adds every object that is owned, directly or through
// other owned objects, by a resource already in the tree.
func attachOwnedResources(tree explorer.ResourceTree, objects []unstructured.Unstructured) explorer.ResourceTree {
	tree.Nodes = slices.Clone(tree.Nodes)
	present := make(map[explorer.ResourceReference]bool, len(tree.Nodes))
	for _, node := range tree.Nodes {
		present[node.Reference()] = true
	}

	for attached := true; attached; {
		attached = false
		for _, object := range objects {
			node := liveNode(object)
			if present[node.Reference()] {
				continue
			}
			if parent, found := knownOwner(object, present); found {
				node.Parents = []explorer.ResourceReference{parent}
				tree.Nodes = append(tree.Nodes, node)
				present[node.Reference()] = true
				attached = true
			}
		}
	}
	return tree
}

func liveNode(object unstructured.Unstructured) explorer.ResourceNode {
	version := object.GroupVersionKind()
	return explorer.ResourceNode{
		Group:     version.Group,
		Version:   version.Version,
		Kind:      version.Kind,
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
		Health:    liveHealth(object),
	}
}

func knownOwner(object unstructured.Unstructured, present map[explorer.ResourceReference]bool) (explorer.ResourceReference, bool) {
	for _, owner := range object.GetOwnerReferences() {
		version, err := schema.ParseGroupVersion(owner.APIVersion)
		if err != nil {
			continue
		}
		reference := explorer.ResourceReference{Group: version.Group, Kind: owner.Kind, Namespace: object.GetNamespace(), Name: owner.Name}
		if present[reference] {
			return reference, true
		}
	}
	return explorer.ResourceReference{}, false
}

// liveHealth approximates Argo CD's health assessment for owned resources.
func liveHealth(object unstructured.Unstructured) string {
	switch object.GetKind() {
	case "Pod":
		return podHealth(object)
	case "ReplicaSet":
		if nestedInt(object.Object, "status", "readyReplicas") >= nestedInt(object.Object, "status", "replicas") {
			return "Healthy"
		}
		return "Progressing"
	case "Job":
		return jobHealth(object)
	default:
		return ""
	}
}

func podHealth(pod unstructured.Unstructured) string {
	switch nestedString(pod.Object, "status", "phase") {
	case "Succeeded":
		return "Healthy"
	case "Failed":
		return "Degraded"
	case "Pending":
		return "Progressing"
	}

	containers, _, _ := unstructured.NestedSlice(pod.Object, "status", "containerStatuses")
	ready := len(containers) > 0
	for _, container := range containers {
		status, ok := container.(map[string]any)
		if !ok {
			continue
		}
		if slices.Contains(failingWaitReasons, nestedString(status, "state", "waiting", "reason")) {
			return "Degraded"
		}
		if isReady, _, _ := unstructured.NestedBool(status, "ready"); !isReady {
			ready = false
		}
	}
	if ready {
		return "Healthy"
	}
	return "Progressing"
}

func jobHealth(job unstructured.Unstructured) string {
	conditions, _, _ := unstructured.NestedSlice(job.Object, "status", "conditions")
	for _, condition := range conditions {
		fields, ok := condition.(map[string]any)
		if !ok || nestedString(fields, "status") != "True" {
			continue
		}
		switch nestedString(fields, "type") {
		case "Failed":
			return "Degraded"
		case "Complete":
			return "Healthy"
		}
	}
	if nestedInt(job.Object, "status", "succeeded") > 0 && nestedInt(job.Object, "status", "active") == 0 {
		return "Healthy"
	}
	return "Progressing"
}

func nestedInt(object map[string]any, fields ...string) int64 {
	value, _, _ := unstructured.NestedInt64(object, fields...)
	return value
}
