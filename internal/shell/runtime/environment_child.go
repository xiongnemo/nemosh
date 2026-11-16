package runtime

import (
	"sort"
	"strings"
)

// childEnviron is the environment a child is given: one of each name as Windows folds its case,
// the canonical spelling of a known name, and otherwise the latest set.
func (e Environment) childEnviron() []string {
	winners := make([]environmentValue, 0, len(e.values))
	for _, candidate := range e.values {
		matched := -1
		for index, winner := range winners {
			if strings.EqualFold(winner.name, candidate.name) {
				matched = index
				break
			}
		}
		if matched < 0 {
			winners = append(winners, candidate)
			continue
		}
		if preferWindowsEnvironmentCandidate(winners[matched], candidate) {
			winners[matched] = candidate
		}
	}
	items := make([]string, 0, len(winners))
	for _, winner := range winners {
		items = append(items, winner.name+"="+winner.value)
	}
	sort.Strings(items)
	return items
}

func preferWindowsEnvironmentCandidate(current, candidate environmentValue) bool {
	canonical, known := canonicalWindowsEnvironmentName(candidate.name)
	if !known {
		return candidate.order > current.order
	}
	currentCanonical := current.name == canonical
	candidateCanonical := candidate.name == canonical
	if currentCanonical != candidateCanonical {
		return candidateCanonical
	}
	return candidate.order > current.order
}
