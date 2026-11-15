package runtime

import (
	"context"
	"testing"
)

// What running a command costs, apart from its words: the bookkeeping around it. Each of these
// was a copy or a new object for every command a loop ran.

// $PIPESTATUS is written over in place when it holds as many statuses as there are now: a
// new array, and a map to keep it in, for every command was nine allocations an iteration
// of a three-command loop.
func TestAllocations_pipeStatusIsWrittenOverInPlace(t *testing.T) {
	rt := expansionRuntime()
	rt.recordPipeStatus(0)

	// When
	count := testing.AllocsPerRun(100, func() { rt.recordPipeStatus(1) })

	// Then
	if count != 0 {
		t.Errorf("%v allocations to record a status where one was", count)
	}
	if got := rt.arrays.indexed["PIPESTATUS"].elements; len(got) != 1 || got[0] != "1" {
		t.Fatalf("PIPESTATUS is %v", got)
	}
	// And a pipeline of another length is an array of its length, as it was.
	rt.recordPipeStatus(1, 0, 3)
	if got := rt.arrays.indexed["PIPESTATUS"].elements; len(got) != 3 || got[0] != "1" || got[1] != "0" || got[2] != "3" {
		t.Fatalf("PIPESTATUS is %v after a pipeline of three", got)
	}
	rt.recordPipeStatus(0)
	if got := rt.arrays.indexed["PIPESTATUS"].elements; len(got) != 1 || got[0] != "0" {
		t.Fatalf("PIPESTATUS is %v after one command", got)
	}
}

// A variable's name is looked up as a name: it was parsed as a positional parameter's number
// first, and the error that said it was not one was an allocation.
func TestAllocations_aNamedParameterIsLookedUpWithoutParsingANumber(t *testing.T) {
	rt := expansionRuntime()
	rt.vars["i"] = "7"
	ctx := context.Background()

	// When
	count := testing.AllocsPerRun(100, func() { rt.lookupParameter(ctx, "i", 0) })

	// Then
	if value, _ := rt.lookupParameter(ctx, "i", 0); value != "7" {
		t.Fatalf("i is %q", value)
	}
	if count != 0 {
		t.Errorf("%v allocations to look up a variable", count)
	}
}

// A variable read in arithmetic costs what its value costs as a number written there.
func TestAllocations_arithmeticReadsAVariableAsItReadsANumber(t *testing.T) {
	rt := expansionRuntime()
	rt.vars["i"] = "7"

	// When
	named := testing.AllocsPerRun(100, func() { _, _ = rt.evaluateArithmetic("i+1") })
	written := testing.AllocsPerRun(100, func() { _, _ = rt.evaluateArithmetic("7+1") })

	// Then
	if value, err := rt.evaluateArithmetic("i+1"); err != nil || value != 8 {
		t.Fatalf("i+1 is %d (%v)", value, err)
	}
	if named > written {
		t.Errorf("reading i cost %v allocations over writing its value", named-written)
	}
}

// A command with nothing in front of it is not copied to find there are no element
// assignments in front of it.
func TestAllocations_aCommandWithNoAssignmentsIsNotCopied(t *testing.T) {
	rt := expansionRuntime()
	ctx := context.Background()
	command := []word{literalWord("true"), literalWord("a")}

	// When
	applied := testing.AllocsPerRun(100, func() { rt.applyArrayAssignments(ctx, command, 0) })
	split := testing.AllocsPerRun(100, func() { splitAssignments([]string{"true", "a"}, 0) })

	// Then
	if applied != 0 || split != 0 {
		t.Errorf("%v allocations looking for array assignments, %v splitting off none", applied, split)
	}
	if assignments, rest := splitAssignments([]string{"true", "a"}, 0); len(assignments) != 0 || len(rest) != 2 {
		t.Fatalf("split into %v and %v", assignments, rest)
	}
}

// An assignment with no tilde in it is not rebuilt looking for one.
func TestAllocations_anAssignmentWithNoTildeIsNotRebuilt(t *testing.T) {
	rt := expansionRuntime()
	item := word{assignmentTilde: true, parts: []wordPart{
		{kind: wordPartLiteral, text: "i=", quote: quoteUnquoted},
		{kind: wordPartArithmetic, text: "i+1", quote: quoteUnquoted},
	}}

	// When
	count := testing.AllocsPerRun(100, func() { rt.tildeParts(item) })

	// Then
	if count != 0 {
		t.Errorf("%v allocations to find no tilde", count)
	}
}
