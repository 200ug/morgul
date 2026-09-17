package tui

import "github.com/sahilm/fuzzy"

// Returns the indices of names ranked by fuzzy match against query (best first).
func fuzzyFind(query string, names []string) []int {
	matches := fuzzy.Find(query, names)
	indices := make([]int, 0, len(matches))
	for _, m := range matches {
		indices = append(indices, m.Index)
	}
	return indices
}
