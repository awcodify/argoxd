package explorer

import "testing"

func TestHierarchyNestsResourcesUnderTheirParents(t *testing.T) {
	tree := ResourceTree{Nodes: []ResourceNode{
		{Kind: "Pod", Name: "web-abc", Parents: []ResourceReference{{Group: "apps", Kind: "ReplicaSet", Name: "web-1"}}},
		{Group: "apps", Kind: "Deployment", Name: "web"},
		{Group: "apps", Kind: "ReplicaSet", Name: "web-1", Parents: []ResourceReference{{Group: "apps", Kind: "Deployment", Name: "web"}}},
		{Kind: "Service", Name: "web"},
	}}

	roots := tree.Hierarchy()

	if len(roots) != 2 || roots[0].Kind != "Deployment" || roots[1].Kind != "Service" {
		t.Fatalf("roots = %+v, want Deployment and Service", roots)
	}
	replicaSets := roots[0].Children
	if len(replicaSets) != 1 || replicaSets[0].Kind != "ReplicaSet" {
		t.Fatalf("deployment children = %+v, want one ReplicaSet", replicaSets)
	}
	pods := replicaSets[0].Children
	if len(pods) != 1 || pods[0].Name != "web-abc" {
		t.Fatalf("replica set children = %+v, want pod web-abc", pods)
	}
}

func TestHierarchyTreatsResourcesWithUnknownParentsAsRoots(t *testing.T) {
	tree := ResourceTree{Nodes: []ResourceNode{
		{Kind: "Pod", Name: "orphan", Parents: []ResourceReference{{Kind: "ReplicaSet", Name: "gone"}}},
	}}

	roots := tree.Hierarchy()

	if len(roots) != 1 || roots[0].Name != "orphan" {
		t.Fatalf("roots = %+v, want orphan pod", roots)
	}
}

func TestHierarchyKeepsEveryResourceWhenParentsFormACycle(t *testing.T) {
	tree := ResourceTree{Nodes: []ResourceNode{
		{Kind: "A", Name: "a", Parents: []ResourceReference{{Kind: "B", Name: "b"}}},
		{Kind: "B", Name: "b", Parents: []ResourceReference{{Kind: "A", Name: "a"}}},
	}}

	if got := countNodes(tree.Hierarchy()); got != 2 {
		t.Fatalf("hierarchy contains %d resources, want 2", got)
	}
}

func countNodes(nodes []ResourceNode) int {
	count := len(nodes)
	for _, node := range nodes {
		count += countNodes(node.Children)
	}
	return count
}

func TestFilterHierarchyKeepsMatchesWithTheirAncestors(t *testing.T) {
	roots := []ResourceNode{
		{Kind: "Deployment", Name: "web", Children: []ResourceNode{
			{Kind: "ReplicaSet", Name: "web-1", Children: []ResourceNode{{Kind: "Pod", Name: "web-1-abc"}}},
		}},
		{Kind: "Service", Name: "web"},
		{Kind: "ConfigMap", Name: "settings"},
	}

	got := FilterHierarchy(roots, "POD")

	if len(got) != 1 || got[0].Kind != "Deployment" {
		t.Fatalf("roots = %+v, want only the Deployment leading to the Pod", got)
	}
	if pod := got[0].Children[0].Children; len(pod) != 1 || pod[0].Name != "web-1-abc" {
		t.Fatalf("path = %+v, want Deployment → ReplicaSet → Pod", got[0])
	}
}

func TestFilterHierarchyMatchesKindAndName(t *testing.T) {
	roots := []ResourceNode{{Kind: "Service", Name: "web"}, {Kind: "Deployment", Name: "web"}}

	if got := FilterHierarchy(roots, "service/web"); len(got) != 1 || got[0].Kind != "Service" {
		t.Fatalf("kind/name filter = %+v, want the Service", got)
	}
	if got := FilterHierarchy(roots, ""); len(got) != 2 {
		t.Fatalf("empty filter = %+v, want every resource", got)
	}
}
