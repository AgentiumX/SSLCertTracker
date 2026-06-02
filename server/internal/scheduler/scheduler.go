package scheduler

func ComputeAgentDomains(globalDomains, includes, excludes []uint) []uint {
	seen := make(map[uint]bool)
	for _, id := range globalDomains {
		seen[id] = true
	}
	for _, id := range includes {
		seen[id] = true
	}
	for _, id := range excludes {
		delete(seen, id)
	}
	result := make([]uint, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	// sort for stable output
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i] > result[j] {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
