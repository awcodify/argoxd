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
