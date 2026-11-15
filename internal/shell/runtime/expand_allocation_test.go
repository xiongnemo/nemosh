package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A word is expanded with the work it asks for and no more. Every word went through brace
// expansion, which split it into an atom per character to look for a brace it did not have,
// and through the field builder, which copied a literal twice to hand back the literal: a
// loop iteration of `[ $i -lt N ]; true; i=$((i+1))` allocated 108 times, about 24 of them
// looking for braces.

func expansionRuntime() Runtime {
	var stdout, stderr bytes.Buffer
	return New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
}

func literalWord(text string) word {
	return word{parts: []wordPart{{kind: wordPartLiteral, text: text, quote: quoteUnquoted}}}
}

// A word that is one unquoted literal with nothing in it to expand is itself: the slice that
// holds it, and nothing else.
func TestAllocations_aPlainWordExpandsToItself(t *testing.T) {
	rt := expansionRuntime()
	ctx := context.Background()
	for _, text := range []string{"true", "-lt", "]", "/c/Users/nemo", "a=b"} {
		item := literalWord(text)

		// When
		count := testing.AllocsPerRun(100, func() { rt.expandCommandWord(ctx, item, 0) })

		// Then
		if got := rt.expandCommandWord(ctx, item, 0); len(got) != 1 || got[0] != text {
			t.Fatalf("%q expanded to %q", text, got)
		}
		if count > 1 {
			t.Errorf("%q: %v allocations to expand to itself, want at most the slice holding it", text, count)
		}
	}
}

// A word with no brace in its unquoted text costs what it costs without brace expansion.
func TestAllocations_aWordWithNoBraceIsNotBraceExpanded(t *testing.T) {
	rt := expansionRuntime()
	rt.vars["i"] = "7"
	ctx := context.Background()
	item := word{parts: []wordPart{
		{kind: wordPartLiteral, text: "prefix-", quote: quoteUnquoted},
		{kind: wordPartParameter, text: "i", quote: quoteUnquoted},
	}}

	// When
	braced := testing.AllocsPerRun(100, func() { rt.expandCommandWord(ctx, item, 0) })
	unbraced := testing.AllocsPerRun(100, func() { rt.expandOneCommandWord(ctx, item, 0) })

	// Then
	if got := rt.expandCommandWord(ctx, item, 0); len(got) != 1 || got[0] != "prefix-7" {
		t.Fatalf("expanded to %q", got)
	}
	if braced > unbraced {
		t.Fatalf("looking for braces in a word with none cost %v allocations over the %v of expanding it", braced-unbraced, unbraced)
	}
}

// The words of a command are tokens, one each, in a list made once.
func TestAllocations_aCommandsPlainWordsAreTokensWithNothingBetween(t *testing.T) {
	rt := expansionRuntime()
	ctx := context.Background()
	words := []word{literalWord("true"), literalWord("a"), literalWord("b"), literalWord("c")}

	// When
	count := testing.AllocsPerRun(100, func() { rt.expandCommandArguments(ctx, words, 0) })

	// Then
	if got := tokenValues(rt.expandCommandArguments(ctx, words, 0)); strings.Join(got, " ") != "true a b c" {
		t.Fatalf("the words came to %q", got)
	}
	if count > 1 {
		t.Fatalf("%v allocations for four plain words, want the one list holding them", count)
	}
}
