package runtime

import "testing"

// What casePatternPosition has to get right, stated as the prefixes the scans actually
// hand it -- each one ends exactly where a `(` or `)` is about to be decided.
func TestCasePatternPosition(t *testing.T) {
	for _, testcase := range []struct {
		name   string
		prefix string
		want   bool
	}{
		{name: "no case at all", prefix: "( echo a ", want: false},
		{name: "the first pattern", prefix: "( case a in a", want: true},
		{name: "the optional leading paren", prefix: "case a in ", want: true},
		{name: "after the pattern ends", prefix: "( case a in a) echo x ", want: false},
		{name: "a subshell in an arm body", prefix: "( case a in a) (echo x", want: false},
		{name: "after a terminator", prefix: "( case a in a) x;; b", want: true},
		{name: "after a terminator with fallthrough", prefix: "( case a in a) x;;& b", want: true},
		{name: "a single semicolon is not a terminator", prefix: "( case a in a) x; y", want: false},
		{name: "after esac", prefix: "( case a in a) x;; esac ", want: false},
		{name: "a pattern alternative", prefix: "case a in a|b", want: true},
		{name: "a nested case's pattern", prefix: "case a in a) case b in b", want: true},
		{name: "the outer arm after a nested esac", prefix: "case a in a) case b in b) x;; esac;; c", want: true},
		{name: "a quoted case is not a case", prefix: `( echo "case a in a" `, want: false},
		{name: "a quoted esac does not close one", prefix: `case a in a) echo "esac";; b`, want: true},
		{name: "the word before in is not a pattern", prefix: "case a", want: false},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if got := casePatternPosition(testcase.prefix); got != testcase.want {
				t.Errorf("casePatternPosition(%q) = %v, want %v", testcase.prefix, got, testcase.want)
			}
		})
	}
}
