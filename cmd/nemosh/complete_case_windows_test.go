//go:build windows

package main

import (
	"strings"
	"testing"
)

// Matching folds case on Windows and the shared prefix did not, so a single
// candidate spelled differently erased the answer.
//
// Measured on a real PATH: `wh` matches eight commands -- who, whoami, whois,
// whois64, WhoUses and three more -- and the shared prefix came back empty
// because WhoUses starts with a capital. Tab therefore inserted nothing and
// listed, while the grey suggestion beside it was already offering `who`. Two
// parts of the same editor disagreeing about the same list is what a user reads
// as "Tab is broken".
//
// busybox-w32 folds case here too, comparing with tolower() under
// ENABLE_PLATFORM_MINGW32 (libbb/lineedit.c:1487-1495).
func TestLongestSharedPrefix_foldsCaseOnWindows(t *testing.T) {
	tests := []struct {
		name    string
		matches []string
		want    string
	}{
		{
			name:    "a capitalised candidate does not erase the prefix",
			matches: []string{"who", "whoami", "whois", "whois64", "WhoUses"},
			want:    "who",
		},
		{
			name:    "the prefix reaches as far as the names agree",
			matches: []string{"whoami", "WhoUses"},
			want:    "who",
		},
		{
			// The spelling comes from the first candidate, which is busybox's
			// rule: it takes the chosen match and truncates it, so the line ends
			// up showing a name as it is actually spelled
			// (libbb/lineedit.c:1483, 1531-1537).
			name:    "the spelling is the first candidate's",
			matches: []string{"WhoUses", "whoami"},
			want:    "Who",
		},
		{
			name:    "names that share nothing still share nothing",
			matches: []string{"who", "ls"},
			want:    "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := longestSharedPrefix(test.matches)

			// Then
			if got != test.want {
				t.Fatalf("longestSharedPrefix(%q) = %q, want %q", test.matches, got, test.want)
			}
		})
	}
}

// A capitalised candidate must not stop Tab where the names still agree.
//
// Before this, `zo` against zoom.exe and ZoomIt.exe inserted nothing: the shared
// prefix was computed byte by byte, `Z` is not `z`, and it collapsed to empty on
// the first comparison. Nothing about the two names was ambiguous to the user.
func TestComplete_advancesPastACapitalisedCandidate(t *testing.T) {
	// Given
	_, editor := newStyledEditor(t, 80, "", nil)
	editor.commands = newShellCommands(settledIndex(t, seedPathDirectory(t, "zoom.exe", "ZoomIt.exe")))

	// When
	for _, r := range "zo" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if got := editor.buffer.String(); got != "zoom" {
		t.Fatalf("buffer = %q, want %q", got, "zoom")
	}
}

// The other half of busybox's rule: a prefix that is the same length but a
// different case is still worth inserting, because it shows the name as it is
// really spelled (libbb/lineedit.c:1531-1537).
func TestComplete_correctsTheCaseOfWhatWasTyped(t *testing.T) {
	// Given
	_, editor := newStyledEditor(t, 80, "", nil)
	editor.commands = newShellCommands(settledIndex(t, seedPathDirectory(t, "ZoomIt.exe", "ZoomBar.exe")))

	// When
	for _, r := range "zoom" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if got := editor.buffer.String(); got != "Zoom" {
		t.Fatalf("buffer = %q, want %q", got, "Zoom")
	}
}

// And the case from the report itself, which is *not* a bug and is pinned so it
// does not get "fixed" into one.
//
// `who` on a real Windows PATH matches five commands -- who, whoami, whois,
// whois64, WhoUses -- and they agree on nothing beyond what was already typed.
// Tab inserting nothing and listing them is the correct answer; the suggestion
// beside it still names one, because picking a single likely winner is a
// different question from finding what every candidate has in common.
func TestComplete_insertsNothingWhenTheCandidatesTrulyDisagree(t *testing.T) {
	// Given
	screen, editor := newStyledEditor(t, 80, "", nil)
	editor.commands = newShellCommands(settledIndex(t,
		seedPathDirectory(t, "who.exe", "whois.exe", "whois64.exe", "WhoUses.exe")))

	// When
	for _, r := range "who" {
		editor.buffer.insert(r)
	}
	editor.complete("$ ")

	// Then
	if got := editor.buffer.String(); got != "who" {
		t.Fatalf("buffer = %q, want it left alone", got)
	}
	// Not silence, though: the choice has to be on screen, or this is
	// indistinguishable from the bug above.
	var rendered strings.Builder
	for row := range 6 {
		rendered.WriteString(screen.text(row))
		rendered.WriteString("\n")
	}
	if !strings.Contains(rendered.String(), "whois64") {
		t.Fatalf("screen = %q, want the candidates listed", rendered.String())
	}
}

// The suggestion had the opposite half of the same bug: it matched by byte, so
// typing WH offered nothing while Tab found eight commands. The two must agree
// about what counts as a match, whichever direction the disagreement runs.
func TestSuggester_foldsCaseForCommandNamesOnWindows(t *testing.T) {
	// Given
	engine := suggester{commands: []string{"whoami", "WhoUses"}}

	// When
	got := engine.suggest("WH")

	// Then
	if got != "oami" {
		t.Fatalf("suggest(%q) = %q, want %q", "WH", got, "oami")
	}
}
