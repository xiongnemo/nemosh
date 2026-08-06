package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The grammar is busybox's prefer_applet_internal (libbb/appletlib.c:296-332),
// reproduced case for case: a lone `-` disables every applet, a lone `+`
// disables one only when an external of that name exists, and otherwise the
// value is a list where names before the first `;` are disabled outright and
// names after it are disabled only when an external exists. Separators are
// space, comma and semicolon -- not tab, and not anything else -- and a name
// only matches when both of its ends land on a separator or the string's end.
func TestPreferApplet_followsTheBusyboxOverrideGrammar(t *testing.T) {
	cases := []struct {
		name     string
		applet   string
		override string
		external bool
		want     bool
	}{
		{name: "unset prefers the applet", applet: "cat", override: "", external: true, want: true},
		{name: "minus disables every applet", applet: "cat", override: "-", external: true, want: false},
		{name: "minus disables even with no external", applet: "cat", override: "-", external: false, want: false},
		{name: "plus yields to an external that exists", applet: "cat", override: "+", external: true, want: false},
		{name: "plus keeps the applet with no external", applet: "cat", override: "+", external: false, want: true},
		{name: "a listed name is disabled outright", applet: "cat", override: "cat", external: false, want: false},
		{name: "an unlisted name is untouched", applet: "ls", override: "cat", external: true, want: true},
		{name: "space separates names", applet: "ls", override: "cat ls", external: false, want: false},
		{name: "comma separates names", applet: "ls", override: "cat,ls", external: false, want: false},
		{name: "after the semicolon an external decides", applet: "ls", override: "cat;ls", external: true, want: false},
		{name: "after the semicolon no external keeps the applet", applet: "ls", override: "cat;ls", external: false, want: true},
		{name: "before the semicolon needs no external", applet: "cat", override: "cat;ls", external: false, want: false},
		{name: "a leading semicolon makes the whole list conditional", applet: "make", override: ";make", external: true, want: false},
		{name: "the first occurrence decides", applet: "make", override: "make;ls,make", external: false, want: false},
		{name: "a bare semicolon lists nothing", applet: "cat", override: ";", external: true, want: true},
		{name: "a substring is not a name", applet: "cat", override: "concat", external: true, want: true},
		{name: "a dot does not end a name", applet: "cat", override: "cat.exe", external: true, want: true},
		{name: "a tab is not a separator", applet: "ls", override: "cat\tls", external: true, want: true},
		{name: "minus is only special alone", applet: "cat", override: "-cat", external: true, want: true},
		{name: "plus is only special alone", applet: "cat", override: "+cat", external: true, want: true},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			externalExists := func(string) bool { return testCase.external }

			// When
			got := preferApplet(testCase.applet, testCase.override, externalExists)

			// Then
			if got != testCase.want {
				t.Fatalf("expected preferApplet(%q, %q) to be %v, got %v", testCase.applet, testCase.override, testCase.want, got)
			}
		})
	}
}

// Searching PATH is the expensive half of the decision, so it happens only for
// the two forms whose answer depends on it.
func TestPreferApplet_searchesForAnExternalOnlyWhenTheAnswerDependsOnOne(t *testing.T) {
	cases := []struct {
		name     string
		override string
		want     int
	}{
		{name: "unset", override: "", want: 0},
		{name: "minus", override: "-", want: 0},
		{name: "unlisted", override: "ls", want: 0},
		{name: "listed outright", override: "cat", want: 0},
		{name: "plus", override: "+", want: 1},
		{name: "listed after the semicolon", override: ";cat", want: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			// Given
			searches := 0
			externalExists := func(string) bool {
				searches++
				return false
			}

			// When
			preferApplet("cat", testCase.override, externalExists)

			// Then
			if searches != testCase.want {
				t.Fatalf("expected %d external searches, got %d", testCase.want, searches)
			}
		})
	}
}

// A shebang naming an applet under a Unix directory is applet lookup too, and
// busybox gates it through the same check (shell/ash.c:6993). With the applet
// overridden and no external of that name anywhere, there is nothing left to
// run the script with.
func TestPlanScriptLaunch_stopsResolvingAnAppletInterpreter_whenItIsOverridden(t *testing.T) {
	// Given
	dir := t.TempDir()
	script := filepath.Join(dir, "show")
	if err := os.WriteFile(script, []byte("#!/bin/cat\nhello\n"), 0o700); err != nil {
		t.Fatalf("write script: %v", err)
	}
	// An empty PATH so that a cat the host happens to ship cannot stand in.
	t.Setenv("PATH", t.TempDir())
	t.Setenv(overrideAppletsVariable, "cat")
	rt := New(applets.DefaultRegistry, Streams{})

	// When
	_, _, err := rt.planScriptLaunch(script, nil, `C:\nemosh\nemosh.exe`)

	// Then
	if err == nil {
		t.Fatalf("expected the overridden applet interpreter to be unresolvable")
	}
	if got := err.Error(); got != "/bin/cat: external command not found" {
		t.Fatalf("expected an external-not-found diagnostic, got %q", got)
	}
}
