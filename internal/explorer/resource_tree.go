package explorer

import "strings"

// Reference returns the identity of a resource node.
func (n ResourceNode) Reference() ResourceReference {
	return ResourceReference{Group: n.Group, Kind: n.Kind, Namespace: n.Namespace, Name: n.Name}
}

// Hierarchy nests each managed resource under its first known parent.
// Resources without a known parent, or caught in a parent cycle, become roots.
func (t ResourceTree) Hierarchy() []ResourceNode {
	present := make(map[ResourceReference]bool, len(t.Nodes))
	for _, node := range t.Nodes {
		present[node.Reference()] = true
	}

	children := make(map[ResourceReference][]ResourceNode)
	var roots []ResourceNode
	for _, node := range t.Nodes {
		if parent, found := firstKnownParent(node, present); found {
			children[parent] = append(children[parent], node)
		} else {
			roots = append(roots, node)
		}
	}

	visited := make(map[ResourceReference]bool, len(t.Nodes))
	var attach func(ResourceNode) ResourceNode
	attach = func(node ResourceNode) ResourceNode {
		visited[node.Reference()] = true
		node.Children = nil
		for _, child := range children[node.Reference()] {
			if !visited[child.Reference()] {
				node.Children = append(node.Children, attach(child))
			}
		}
		return node
	}

	hierarchy := make([]ResourceNode, 0, len(roots))
	for _, root := range roots {
		hierarchy = append(hierarchy, attach(root))
	}
	for _, node := range t.Nodes {
		if !visited[node.Reference()] {
			hierarchy = append(hierarchy, attach(node))
		}
	}
	return hierarchy
}

func firstKnownParent(node ResourceNode, present map[ResourceReference]bool) (ResourceReference, bool) {
	for _, parent := range node.Parents {
		if present[parent] {
			return parent, true
		}
	}
	return ResourceReference{}, false
}

// EffectiveSync is the resource's own sync status or, for resources Argo CD
// does not track itself (such as the ReplicaSets and Pods a Deployment owns),
// the status inherited from their owner.
func (n ResourceNode) EffectiveSync(inherited string) string {
	if n.Sync != "" {
		return n.Sync
	}
	return inherited
}

// Matches reports whether the resource's "kind/name" contains the search text,
// ignoring case, and the resource passes the filter. The inherited sync status
// is the owner's effective status.
func (n ResourceNode) Matches(search string, filter Filter, inheritedSync string) bool {
	return strings.Contains(strings.ToLower(n.Kind+"/"+n.Name), strings.ToLower(search)) &&
		filter.Matches(n.EffectiveSync(inheritedSync), n.Health, n.Kind)
}

// FilterHierarchy keeps the resources that match the search and filter together
// with the resources on their path, so a match is never shown without its owners.
// The inherited sync status is that of the Application the roots belong to.
func FilterHierarchy(roots []ResourceNode, search string, filter Filter, inheritedSync string) []ResourceNode {
	if search == "" && !filter.Active() {
		return roots
	}
	return filterBranch(roots, search, filter, inheritedSync)
}

func filterBranch(nodes []ResourceNode, search string, filter Filter, inheritedSync string) []ResourceNode {
	var kept []ResourceNode
	for _, node := range nodes {
		children := filterBranch(node.Children, search, filter, node.EffectiveSync(inheritedSync))
		if node.Matches(search, filter, inheritedSync) || len(children) > 0 {
			node.Children = children
			kept = append(kept, node)
		}
	}
	return kept
}
