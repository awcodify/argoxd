package argocd

import "strings"

// lineDiff compares two texts line by line. Every line is prefixed with
// "  " when unchanged, "- " when only in before, or "+ " when only in after.
func lineDiff(before, after string) []string {
	a, b := splitLines(before), splitLines(after)

	// common[i][j] is the length of the longest common subsequence of a[i:] and b[j:].
	common := make([][]int, len(a)+1)
	for i := range common {
		common[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				common[i][j] = common[i+1][j+1] + 1
			} else {
				common[i][j] = max(common[i+1][j], common[i][j+1])
			}
		}
	}

	lines := make([]string, 0, max(len(a), len(b)))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			lines = append(lines, "  "+a[i])
			i++
			j++
		case common[i+1][j] >= common[i][j+1]:
			lines = append(lines, "- "+a[i])
			i++
		default:
			lines = append(lines, "+ "+b[j])
			j++
		}
	}
	for ; i < len(a); i++ {
		lines = append(lines, "- "+a[i])
	}
	for ; j < len(b); j++ {
		lines = append(lines, "+ "+b[j])
	}
	return lines
}

func splitLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}
