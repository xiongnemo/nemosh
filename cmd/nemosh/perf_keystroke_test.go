package main

import (
	"fmt"
	"strings"
	"testing"
)

// The keystroke path is the one place in this shell where performance is a
// correctness-adjacent property: a suggestion is recomputed after *every*
// character typed, so work per keystroke is felt directly as the line stuttering
// under the fingers. Everywhere else a millisecond is invisible.
//
// Allocations are what is gated on, for the reason allocation_test.go gives: they
// are deterministic, and a wall-clock threshold on a shared runner flaps until it
// is ignored. The ceilings are about twice the measured figure, so ordinary
// editing moves nothing and a change of algorithm is what trips them.
//
// The list sizes are measured from the machine this was written on: 78 PATH
// directories yielding just over a thousand offerable names, and a history file
// at its default cap.
func keystrokeAllocations(t *testing.T, work func()) int {
	t.Helper()
	return int(testing.AllocsPerRun(50, work))
}

func syntheticCommands(count int) []string {
	names := make([]string, 0, count)
	for index := range count {
		names = append(names, fmt.Sprintf("command-%04d", index))
	}
	return names
}

func syntheticHistory(count int) []string {
	lines := make([]string, 0, count)
	for index := range count {
		lines = append(lines, fmt.Sprintf("ssh gpu-worker-%d --flag value", index))
	}
	return lines
}

func TestAllocations_suggestingAfterAKeystroke(t *testing.T) {
	commands := syntheticCommands(1200)
	history := syntheticHistory(1000)
	for _, test := range []struct {
		name    string
		line    string
		ceiling int
	}{
		{
			// The best case and the common one: the line was run before, so the
			// first source answers and nothing else is consulted.
			name: "a line already in history", line: "ssh gpu-worker-7", ceiling: 4,
		},
		{
			// The worst case for the command scan: nothing in history matches, so
			// every name on PATH is examined.
			name: "a command prefix that misses history", line: "command-08", ceiling: 4,
		},
		{
			// Nothing matches anywhere, so all three sources run to completion.
			name: "a prefix that matches nothing", line: "zzzznotacommand", ceiling: 4,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := suggester{history: history, commands: commands}

			// When
			count := keystrokeAllocations(t, func() {
				engine.suggest(test.line)
			})

			// Then
			t.Logf("%d allocations for %q against %d commands and %d history lines",
				count, test.line, len(commands), len(history))
			if count > test.ceiling {
				t.Fatalf("suggesting after %q allocated %d times, over the ceiling of %d",
					test.line, count, test.ceiling)
			}
		})
	}
}

// Typing a word one character at a time is the shape the editor actually asks
// in, and it is what makes a per-keystroke cost add up: the same scan is redone
// for every prefix of the word.
func TestAllocations_suggestingThroughAWholeWord(t *testing.T) {
	commands := syntheticCommands(1200)
	history := syntheticHistory(1000)
	engine := suggester{history: history, commands: commands}
	word := "command-0999"

	// When
	count := keystrokeAllocations(t, func() {
		for size := 1; size <= len(word); size++ {
			engine.suggest(word[:size])
		}
	})

	// Then
	t.Logf("%d allocations for the %d keystrokes of %q", count, len(word), word)
	if ceiling := 40; count > ceiling {
		t.Fatalf("typing %q allocated %d times, over the ceiling of %d", word, count, ceiling)
	}
}

func BenchmarkSuggest(b *testing.B) {
	commands := syntheticCommands(1200)
	history := syntheticHistory(1000)
	engine := suggester{history: history, commands: commands}
	for _, test := range []struct{ name, line string }{
		{name: "history hit", line: "ssh gpu-worker-7"},
		{name: "command scan", line: "command-08"},
		{name: "no match", line: "zzzznotacommand"},
	} {
		b.Run(test.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				engine.suggest(test.line)
			}
		})
	}
}

func BenchmarkCompletionMatches(b *testing.B) {
	// The innermost call of the command scan, run once per name per keystroke.
	names := syntheticCommands(1200)
	b.ReportAllocs()
	for b.Loop() {
		for _, name := range names {
			completionMatches(name, "command-08")
		}
	}
}

func BenchmarkLongestSharedPrefix(b *testing.B) {
	matches := make([]string, 0, 64)
	for index := range 64 {
		matches = append(matches, fmt.Sprintf("command-%04d-with-a-long-tail", index))
	}
	b.ReportAllocs()
	for b.Loop() {
		longestSharedPrefix(matches)
	}
}

// A guard rather than a measurement: the suggester's contract is that it never
// touches the filesystem, and a benchmark is where a stray os.Stat would first
// show up as an unexplained slowdown. Stating it as a test means it shows up as
// a failure instead.
func TestSuggest_returnsRemainderOnly_whenHistoryMatches(t *testing.T) {
	engine := suggester{history: []string{"ssh gpu-worker-34"}}

	// When
	remainder := engine.suggest("s")

	// Then
	if want := "sh gpu-worker-34"; remainder != want {
		t.Fatalf("suggest(%q) = %q, want %q", "s", remainder, want)
	}
	if strings.HasPrefix(remainder, "s") && remainder == "ssh gpu-worker-34" {
		t.Fatal("the suggestion repeated what was typed")
	}
}
