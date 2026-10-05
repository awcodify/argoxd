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

	got := FilterHierarchy(roots, "POD", Filter{}, "")

	if len(got) != 1 || got[0].Kind != "Deployment" {
		t.Fatalf("roots = %+v, want only the Deployment leading to the Pod", got)
	}
	if pod := got[0].Children[0].Children; len(pod) != 1 || pod[0].Name != "web-1-abc" {
		t.Fatalf("path = %+v, want Deployment → ReplicaSet → Pod", got[0])
	}
}

func TestFilterHierarchyMatchesKindAndName(t *testing.T) {
	roots := []ResourceNode{{Kind: "Service", Name: "web"}, {Kind: "Deployment", Name: "web"}}

	if got := FilterHierarchy(roots, "service/web", Filter{}, ""); len(got) != 1 || got[0].Kind != "Service" {
		t.Fatalf("kind/name search = %+v, want the Service", got)
	}
	if got := FilterHierarchy(roots, "", Filter{}, ""); len(got) != 2 {
		t.Fatalf("empty search = %+v, want every resource", got)
	}
}

func TestFilterHierarchyFiltersByStatusAndKind(t *testing.T) {
	roots := []ResourceNode{
		{Kind: "Deployment", Name: "web", Health: "Healthy", Children: []ResourceNode{
			{Kind: "Pod", Name: "web-1", Health: "Degraded"},
			{Kind: "Pod", Name: "web-2", Health: "Healthy"},
		}},
		{Kind: "Service", Name: "web", Sync: "OutOfSync"},
	}

	got := FilterHierarchy(roots, "", Filter{Health: "Degraded"}, "")
	if len(got) != 1 || len(got[0].Children) != 1 || got[0].Children[0].Name != "web-1" {
		t.Fatalf("degraded = %+v, want the Deployment leading to web-1", got)
	}
	if got := FilterHierarchy(roots, "", Filter{Sync: "OutOfSync"}, ""); len(got) != 1 || got[0].Kind != "Service" {
		t.Fatalf("out of sync = %+v, want the Service", got)
	}
	if got := FilterHierarchy(roots, "web-2", Filter{Kind: "pod"}, ""); len(got) != 1 || len(got[0].Children) != 1 {
		t.Fatalf("search and kind = %+v, want only web-2 under its Deployment", got)
	}
}

func TestResourceTreeKindsAreDistinctAndSorted(t *testing.T) {
	tree := ResourceTree{Nodes: []ResourceNode{{Kind: "Service"}, {Kind: "Pod"}, {Kind: "Pod"}, {}}}
	if got := tree.Kinds(); len(got) != 2 || got[0] != "Pod" || got[1] != "Service" {
		t.Fatalf("kinds = %v, want [Pod Service]", got)
	}
}

func TestFilterHierarchyInheritsSyncStatusFromTheOwner(t *testing.T) {
	roots := []ResourceNode{
		{Kind: "Deployment", Name: "web", Sync: "Synced", Children: []ResourceNode{
			{Kind: "ReplicaSet", Name: "web-1", Children: []ResourceNode{{Kind: "Pod", Name: "web-1-abc"}}},
		}},
		{Kind: "Service", Name: "web", Sync: "OutOfSync"},
	}

	got := FilterHierarchy(roots, "", Filter{Sync: "Synced"}, "")
	if len(got) != 1 || len(got[0].Children) != 1 || len(got[0].Children[0].Children) != 1 {
		t.Fatalf("synced = %+v, want the Deployment with its ReplicaSet and Pod", got)
	}
	if got := FilterHierarchy(roots, "", Filter{Sync: "OutOfSync"}, ""); len(got) != 1 || got[0].Kind != "Service" {
		t.Fatalf("out of sync = %+v, want only the Service", got)
	}
}
