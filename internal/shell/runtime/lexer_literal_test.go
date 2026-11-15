package runtime

import (
	"strings"
	"testing"
)

// A word's literal text is built a byte at a time, and each byte was added with `+=`: a copy
// of the word so far, so a word of N characters cost N allocations and N squared bytes. It was
// the largest single cost of parsing git-completion.bash, a third of the bytes allocated.

// Twice as long a word is no more work to lex than the word, by allocation: the run it is
// taken from is sliced, not copied a byte at a time.
func TestAllocations_aLongWordCostsWhatAShortOneDoes(t *testing.T) {
	short := "echo " + strings.Repeat("a", 200) + "\n"
	long := "echo " + strings.Repeat("a", 400) + "\n"

	// When
	shortCost := allocations(t, func() { _, _ = ParseScript(short) })
	longCost := allocations(t, func() { _, _ = ParseScript(long) })

	// Then
	t.Logf("200 characters: %d allocations, 400: %d", shortCost, longCost)
	if longCost > shortCost+8 {
		t.Fatalf("twice the word took %d allocations against %d: the word is copied as it grows", longCost, shortCost)
	}
}

// What is lexed is what was written, wherever the run of a word is broken: by quotes, an
// escape, an expansion, or a multi-byte character, whose bytes arrive one at a time.
func TestLexer_aWordsLiteralRunsAreTheTextWritten(t *testing.T) {
	for _, test := range []struct{ source, want string }{
		{`echo abc"def"ghi`, "abcdefghi"},
		{`echo a\bc`, "abc"},
		{`echo pre${x}post`, "prepost"},
		{`echo 中文abc`, "中文abc"},
		{`echo 'it''s'`, "its"},
	} {
		var stdout strings.Builder
		rt := expansionRuntime()
		rt.streams.Stdout = &stdout
		if status := rt.RunScript(t.Context(), test.source+"\n"); status != 0 || stdout.String() != test.want+"\n" {
			t.Errorf("%s printed %q, status %d; want %q", test.source, stdout.String(), status, test.want)
		}
	}
}
