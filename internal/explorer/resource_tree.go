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

// MatchesFilter reports whether the resource's "kind/name" contains the query, ignoring case.
func (n ResourceNode) MatchesFilter(query string) bool {
	return strings.Contains(strings.ToLower(n.Kind+"/"+n.Name), strings.ToLower(query))
}

// FilterHierarchy keeps the resources that match the query together with the
// resources on their path, so a match is never shown without its owners.
func FilterHierarchy(roots []ResourceNode, query string) []ResourceNode {
	if query == "" {
		return roots
	}
	var kept []ResourceNode
	for _, node := range roots {
		children := FilterHierarchy(node.Children, query)
		if node.MatchesFilter(query) || len(children) > 0 {
			node.Children = children
			kept = append(kept, node)
		}
	}
	return kept
}
