//go:build !windows

package main

import "testing"

// The mirror of complete_case_windows_test.go. Case is meaningful here, so two
// spellings are two different names and share only what they literally share --
// the same reason completionMatches compares bytes on this side of the split.
func TestLongestSharedPrefix_keepsCaseOffWindows(t *testing.T) {
	// When
	got := longestSharedPrefix([]string{"whoami", "WhoUses"})

	// Then
	if got != "" {
		t.Fatalf("longestSharedPrefix = %q, want %q: Makefile and makefile are two files", got, "")
	}
}

func TestSuggester_keepsCaseForCommandNamesOffWindows(t *testing.T) {
	// Given
	engine := suggester{commands: []string{"whoami", "WhoUses"}}

	// When
	got := engine.suggest("WH")

	// Then
	if got != "" {
		t.Fatalf("suggest(%q) = %q, want nothing", "WH", got)
	}
}

// True on both platforms, and the property that was actually violated: whatever
// Tab inserts has to be something the user could have typed toward. A shared
// prefix shorter than the stem means the editor disagrees with its own match
// rule, which is how the Windows bug showed itself.
func TestLongestSharedPrefix_neverFallsShortOfTheStem(t *testing.T) {
	// Given
	stem := "wh"
	candidates := []string{"who", "whoami", "whois"}

	// When
	shared := longestSharedPrefix(candidates)

	// Then
	if !completionMatches(shared, stem) {
		t.Fatalf("shared prefix %q does not itself match the stem %q", shared, stem)
	}
}
