package runtime

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// `${v//0/1}` of a pattern with nothing of a pattern's in it is a search for the text, as bash's
// and busybox's are cheap: it tried the pattern from every place to every place after it, each try
// making both into runes, so a value of 3,000 characters took 6.3 s and 224 MiB of commit, where
// busybox takes 41 ms.
func TestParameterReplace_ofALiteralSearchesForIt(t *testing.T) {
	value := strings.Repeat("0", 3000)
	allocations := testing.AllocsPerRun(5, func() {
		if got := parameterReplace(value, "//", "0/1", false); got != strings.Repeat("1", 3000) {
			t.Fatalf("${v//0/1} gave %.20q...", got)
		}
	})
	if allocations > 10 {
		t.Fatalf("${v//0/1} of 3,000 characters made %v allocations, want a handful", allocations)
	}
}

// And it answers what the pattern matcher answers, for each operator, folded or not, whatever the
// characters: the reference is the scan it replaced, made of the same helpers.
func TestParameterReplace_literalAgreesWithThePatternScan(t *testing.T) {
	random := rand.New(rand.NewPCG(11, 12))
	pieces := []string{"a", "b", "ab", "A", "é", "中", "-", " ", "aa", "x"}
	word := func(most int) string {
		var text strings.Builder
		for range random.IntN(most) {
			text.WriteString(pieces[random.IntN(len(pieces))])
		}
		return text.String()
	}
	for range 4000 {
		value, pattern, replacement := word(9), word(3), word(2)
		if pattern == "" {
			continue
		}
		operator := []string{"/", "//", "/#", "/%"}[random.IntN(4)]
		fold := random.IntN(4) == 0
		want := scannedReplace(value, operator, pattern, replacement, fold)
		if got := parameterReplace(value, operator, pattern+"/"+replacement, fold); got != want {
			t.Fatalf("${v%s%s/%s} of %q, fold %v: %q, the scan gives %q", operator, pattern, replacement, value, fold, got, want)
		}
	}
}

// scannedReplace is parameterReplace as it was for a pattern that is not empty.
func scannedReplace(value, operator, pattern, replacement string, fold bool) string {
	subject := value
	if fold {
		subject, pattern = foldASCII(value), foldASCII(pattern)
	}
	characters := cutsCharacters(subject, pattern)
	switch operator {
	case "/#":
		if width, ok := longestPatternMatchAt(subject, pattern, 0, characters); ok {
			return replacement + value[width:]
		}
		return value
	case "/%":
		for start := 0; start <= len(subject); start++ {
			if isCut(subject, start, characters) && matchShellPattern(pattern, subject[start:]) {
				return value[:start] + replacement
			}
		}
		return value
	}
	return replaceMatches(value, subject, pattern, replacement, operator == "//", characters)
}
