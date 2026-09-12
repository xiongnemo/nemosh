package runtime

import (
	"strings"
	"testing"
)

// HISTCONTROL, HISTSIZE, and why they matter more than their size suggests.
//
// **A leading space is how everyone keeps a token out of their history.** Both variables
// were settable and had no effect whatever, so ` export GITHUB_TOKEN=...` silently wrote
// the secret to the file on disk. That is worse than not offering the feature at all: a
// refusal would have been noticed, and a habit that does nothing is trusted anyway.

func TestParseHistoryControl(t *testing.T) {
	for _, test := range []struct {
		value string
		want  historyControl
	}{
		{value: "", want: historyControl{}},
		{value: "ignorespace", want: historyControl{ignoreSpace: true}},
		{value: "ignoredups", want: historyControl{ignoreDups: true}},
		{value: "ignoreboth", want: historyControl{ignoreSpace: true, ignoreDups: true}},
		{value: "erasedups", want: historyControl{eraseDups: true}},
		// Colon-separated, in any order, and combinable.
		{value: "ignorespace:erasedups", want: historyControl{ignoreSpace: true, eraseDups: true}},
		{value: "erasedups:ignoredups", want: historyControl{ignoreDups: true, eraseDups: true}},
		// An unknown word is ignored rather than refused, which is bash's behaviour and
		// the safe direction: an rc file shared with bash must not stop this shell
		// starting because it names a setting this one has not got.
		{value: "ignorespace:nosuchsetting", want: historyControl{ignoreSpace: true}},
		{value: "nosuchsetting", want: historyControl{}},
	} {
		t.Run(test.value, func(t *testing.T) {
			if got := parseHistoryControl(test.value); got != test.want {
				t.Fatalf("parseHistoryControl(%q) = %+v, want %+v", test.value, got, test.want)
			}
		})
	}
}

// recordHistoryLine reads the *raw* line, because by the time one has been through the
// parser there is no leading space left to notice. That ordering is the whole subtlety.
func TestRecordHistoryLine(t *testing.T) {
	for _, test := range []struct {
		name    string
		control string
		size    string
		lines   []string
		want    []string
	}{
		{
			name: "a leading space is not recorded", control: "ignorespace",
			lines: []string{"echo one", " echo secret", "echo two"},
			want:  []string{"echo one", "echo two"},
		},
		{
			name: "without ignorespace it is", control: "",
			lines: []string{" echo secret"},
			want:  []string{" echo secret"},
		},
		{
			name: "a repeat is skipped", control: "ignoredups",
			lines: []string{"echo a", "echo a", "echo b"},
			want:  []string{"echo a", "echo b"},
		},
		{
			// Only an *immediate* repeat: `a b a` keeps all three, which is what
			// distinguishes ignoredups from erasedups.
			name: "ignoredups is only the immediate repeat", control: "ignoredups",
			lines: []string{"echo a", "echo b", "echo a"},
			want:  []string{"echo a", "echo b", "echo a"},
		},
		{
			// erasedups removes the earlier copies, so the newest wins its place.
			name: "erasedups keeps the latest", control: "erasedups",
			lines: []string{"echo a", "echo b", "echo a"},
			want:  []string{"echo b", "echo a"},
		},
		{
			name: "ignoreboth is both", control: "ignoreboth",
			lines: []string{"echo a", "echo a", " echo secret", "echo b"},
			want:  []string{"echo a", "echo b"},
		},
		{
			// HISTSIZE keeps the *newest*, which is the only useful direction: a
			// history that dropped what you just ran would make the arrows useless at
			// exactly the moment they are wanted.
			name: "HISTSIZE keeps the newest", size: "2",
			lines: []string{"echo a", "echo b", "echo c"},
			want:  []string{"echo b", "echo c"},
		},
		{
			name: "HISTSIZE=0 keeps nothing", size: "0",
			lines: []string{"echo a", "echo b"},
			want:  nil,
		},
		{
			// A value that is not a number falls back rather than refusing, for the
			// same reason an unknown HISTCONTROL word is ignored.
			name: "a bad HISTSIZE falls back", size: "not-a-number",
			lines: []string{"echo a", "echo b"},
			want:  []string{"echo a", "echo b"},
		},
		{
			name:  "a blank line is never recorded",
			lines: []string{"echo a", "   ", "", "echo b"},
			want:  []string{"echo a", "echo b"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := Runtime{vars: map[string]string{}, history: newShellHistory(), mutatedVars: map[string]struct{}{}}
			if test.control != "" {
				r.vars["HISTCONTROL"] = test.control
			}
			if test.size != "" {
				r.vars["HISTSIZE"] = test.size
			}
			for _, line := range test.lines {
				r.recordHistoryLine(line)
			}
			got := r.history.list()
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Fatalf("history\n  got  %q\n  want %q", got, test.want)
			}
		})
	}
}

// HISTSIZE never exceeds the built-in ceiling, so a rc file cannot make one session hold
// an unbounded list.
func TestHistoryLimit_isCapped(t *testing.T) {
	r := Runtime{vars: map[string]string{"HISTSIZE": "99999999"}}
	if got := r.historyLimit(); got != maxHistoryEntries {
		t.Fatalf("historyLimit = %d, want the ceiling %d", got, maxHistoryEntries)
	}
	// Unset is the ceiling too, which is what keeps the old behaviour for anyone who
	// never sets it.
	empty := Runtime{vars: map[string]string{}}
	if got := empty.historyLimit(); got != maxHistoryEntries {
		t.Fatalf("historyLimit with no HISTSIZE = %d, want %d", got, maxHistoryEntries)
	}
	// And a negative value falls back rather than truncating to nothing.
	negative := Runtime{vars: map[string]string{"HISTSIZE": "-5"}}
	if got := negative.historyLimit(); got != maxHistoryEntries {
		t.Fatalf("historyLimit with a negative HISTSIZE = %d, want %d", got, maxHistoryEntries)
	}
}
