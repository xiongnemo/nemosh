package runtime

import (
	"reflect"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An expression evaluated again is not split into tokens again: a loop's `i+1` costs nothing
// to read after its first turn.
func TestArithmetic_anExpressionAgainIsNotTokenizedAgain(t *testing.T) {
	rt := New(applets.DefaultRegistry, Streams{})
	rt.vars["i"] = "41"
	if value, err := rt.evaluateArithmetic("i+1"); err != nil || value != 42 {
		t.Fatalf("i+1 = %d, %v", value, err)
	}

	// When
	allocations := testing.AllocsPerRun(100, func() {
		if value, err := rt.evaluateArithmetic("i+1"); err != nil || value != 42 {
			t.Fatalf("i+1 = %d, %v", value, err)
		}
	})

	// Then
	if allocations != 0 {
		t.Fatalf("evaluating i+1 again allocated %v times", allocations)
	}
}

// The tokens handed out again are the tokens the text has, whatever else shares its slot.
func TestArithmetic_cachedTokensAreTheTextsOwn(t *testing.T) {
	for round := range 2 {
		for _, expression := range []string{"i+1", "x = y ? 2 : 3", "a[i]++", "2#101 << 3", "i+1 ", "-(-1)"} {
			if got, want := arithmeticTokensOf(expression), tokenizeArithmetic(expression); !reflect.DeepEqual(got, want) {
				t.Errorf("round %d: %q came back as %q, want %q", round, expression, got, want)
			}
		}
	}
}
