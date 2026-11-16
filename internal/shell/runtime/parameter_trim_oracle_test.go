package runtime

import (
	"math/rand/v2"
	"strings"
	"testing"
)

// The trims as they were written before the pattern was converted once a trim and the common
// shapes searched for: every cut point matched afresh, which made `${p##*/}` of an 80-character
// path cost 20 µs where busybox's costs nothing to speak of. Kept verbatim as the oracle the new
// ones answer to, as case_scan_oracle_test.go keeps the case questions.

func trimPatternPrefixAsWritten(value, pattern string, longest bool) string {
	best, characters := -1, cutsCharacters(value, pattern)
	for end := 0; end <= len(value); end++ {
		if !isCut(value, end, characters) || !matchShellPattern(pattern, value[:end]) {
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

func trimPatternSuffixAsWritten(value, pattern string, longest bool) string {
	best, characters := -1, cutsCharacters(value, pattern)
	for start := len(value); start >= 0; start-- {
		if !isCut(value, start, characters) || !matchShellPattern(pattern, value[start:]) {
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

// trimAtoms are what the patterns are made of: every special character and its escape, a bracket
// expression, a class, a character of two bytes and one that is no UTF-8, and the extended
// operators.
var trimAtoms = []string{"*", "?", "a", "/", ".", `\*`, `\\`, `\`, "[a-c]", "[!a]", "[", "]", "[[:alpha:]]", "μ", "\xff", "@(a|b)", "!(a)", "+(a)", "*(b)"}

// trimLetters are what the values are made of.
var trimLetters = []string{"a", "b", "/", ".", "*", `\`, "[", "μ", "\xff"}

func checkTrims(t *testing.T, value, pattern string) bool {
	t.Helper()
	for _, operator := range []string{"#", "##", "%", "%%"} {
		var want string
		longest := len(operator) == 2
		if operator[0] == '#' {
			want = trimPatternPrefixAsWritten(value, pattern, longest)
		} else {
			want = trimPatternSuffixAsWritten(value, pattern, longest)
		}
		if got := trimParameter(operator, value, pattern); got != want {
			t.Errorf("${v%s%s} of %q is %q, as written %q", operator, pattern, value, got, want)
			return false
		}
	}
	return true
}

// Every pattern of up to two atoms against every value of up to three letters, four operators
// each: 400 patterns by 820 values.
func TestTrim_answersAsItWasWrittenEverywhere(t *testing.T) {
	patterns := []string{""}
	for _, first := range trimAtoms {
		patterns = append(patterns, first)
		for _, second := range trimAtoms {
			patterns = append(patterns, first+second)
		}
	}
	values := []string{""}
	for length := 1; length <= 3; length++ {
		next := values
		for _, value := range values {
			if len([]rune(value)) == length-1 {
				for _, letter := range trimLetters {
					next = append(next, value+letter)
				}
			}
		}
		values = next
	}
	for _, pattern := range patterns {
		for _, value := range values {
			if !checkTrims(t, value, pattern) {
				return
			}
		}
	}
}

// Longer values and patterns of up to five atoms, drawn from a fixed seed, and the paths trims
// are mostly asked of.
func TestTrim_answersAsItWasWrittenForLongerOnes(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	for range 20000 {
		var pattern, value strings.Builder
		for range random.IntN(5) + 1 {
			pattern.WriteString(trimAtoms[random.IntN(len(trimAtoms))])
		}
		for range random.IntN(12) {
			value.WriteString(trimLetters[random.IntN(len(trimLetters))])
		}
		if !checkTrims(t, value.String(), pattern.String()) {
			return
		}
	}
	for _, path := range []string{"/c/Users/nemo/x.tar.gz", "a/b/c.txt", "dir/", "/", "no-slash", "a//b", "μ/ν.ξ"} {
		for _, pattern := range []string{"*/", "/*", "*.", ".*", "*", "", "*/*", "x", `\/*`, "*.tar.gz", "/c/*"} {
			if !checkTrims(t, path, pattern) {
				return
			}
		}
	}
}

// parameterOperatorAtAsWritten is how the operator of `${name...}` was found: each of 23
// operators tried at each byte of the name, which was most of what a short trim cost.
func parameterOperatorAtAsWritten(body string, index int) string {
	for _, operator := range [...]string{
		":-", ":=", ":+", ":?", "##", "%%", "//", "/#", "/%", "^^", ",,", "~~",
		":", "-", "=", "+", "?", "#", "%", "/", "^", ",", "~",
	} {
		if strings.HasPrefix(body[index:], operator) {
			return operator
		}
	}
	return ""
}

// Every body of up to three bytes from the operators' characters and a letter, at each index.
func TestParameterOperatorAt_answersAsItWasWritten(t *testing.T) {
	letters := ":-=+?#%/^,~a["
	bodies := []string{""}
	for range 3 {
		for _, body := range bodies {
			for index := range len(letters) {
				bodies = append(bodies, body+letters[index:index+1])
			}
		}
	}
	for _, body := range bodies {
		for index := range len(body) {
			if got, want := parameterOperatorAt(body, index), parameterOperatorAtAsWritten(body, index); got != want {
				t.Fatalf("operator at %d of %q is %q, as written %q", index, body, got, want)
			}
		}
	}
}

// The shapes nearly every trim has -- `*/`, `.*`, a literal -- are a search of the value, which
// costs no allocation however long the value is; a bracket is matched with the value converted
// once, so twice its length costs no more.
func TestAllocations_aTrimOfACommonShapeAllocatesNothing(t *testing.T) {
	value := strings.Repeat("dir/", 50) + "f.tar.gz"
	for _, test := range []struct {
		operator, pattern string
		most              float64
	}{
		{"##", "*/", 0}, {"#", "*/", 0}, {"%", ".*", 0}, {"%%", ".*", 0}, {"%%", "/*", 0}, {"#", "dir/", 0}, {"%", ".gz", 0},
	} {
		if got := testing.AllocsPerRun(50, func() { _ = trimParameter(test.operator, value, test.pattern) }); got > test.most {
			t.Errorf("${v%s%s} allocated %.0f times, want at most %.0f", test.operator, test.pattern, got, test.most)
		}
	}
	short, long := strings.Repeat("dir/", 20)+"f", strings.Repeat("dir/", 200)+"f"
	shortCost := testing.AllocsPerRun(20, func() { _ = trimParameter("##", short, "[d]*/") })
	longCost := testing.AllocsPerRun(20, func() { _ = trimParameter("##", long, "[d]*/") })
	if longCost > shortCost {
		t.Errorf("${v##[d]*/} allocated %.0f times for a value ten times as long, and %.0f for the short one", longCost, shortCost)
	}
}
