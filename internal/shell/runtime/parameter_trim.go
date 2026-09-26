package runtime

// trimParameter is the #, ##, % and %% family: strip the shortest or longest
// matching prefix or suffix, where "matching" is the pattern language of 2.13.1
// rather than a literal comparison.
func trimParameter(operator, value, pattern string) string {
	switch operator {
	case "#":
		return trimPatternPrefix(value, pattern, false)
	case "##":
		return trimPatternPrefix(value, pattern, true)
	case "%":
		return trimPatternSuffix(value, pattern, false)
	default:
		return trimPatternSuffix(value, pattern, true)
	}
}

func trimPatternPrefix(value, pattern string, longest bool) string {
	best := -1
	for end := 0; end <= len(value); end++ {
		if !matchShellPattern(pattern, value[:end]) {
			continue
		}
		best = end
		if !longest {
			break
		}
	}
	if best < 0 {
		return value
	}
	return value[best:]
}

func trimPatternSuffix(value, pattern string, longest bool) string {
	best := -1
	for start := len(value); start >= 0; start-- {
		if !matchShellPattern(pattern, value[start:]) {
			continue
		}
		best = start
		if !longest {
			break
		}
	}
	if best < 0 {
		return value
	}
	return value[:best]
}
