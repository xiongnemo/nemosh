package runtime

import (
	"fmt"
	"strings"
	"testing"
)

// The parse is linear in what is inside a group: twice the arms, or twice the definitions, is
// twice the work. Each bracket read the group from its start, so a case of N arms in a
// function cost N squared; measured, 100 arms allocated a quarter of what 200 did.
func TestAllocations_parsingIsLinearInsideAGroup(t *testing.T) {
	for _, test := range []struct {
		name string
		make func(count int) string
	}{
		{name: "a case's arms in a function", make: func(count int) string {
			var b strings.Builder
			b.WriteString("f() {\ncase $x in\n")
			for index := range count {
				fmt.Fprintf(&b, "  a%d) echo %d ;;\n", index, index)
			}
			b.WriteString("esac\n}\n")
			return b.String()
		}},
		{name: "cases in a brace group", make: func(count int) string {
			return "{\n" + strings.Repeat("case $x in a) echo a ;; *) echo b ;; esac\n", count) + "}\n"
		}},
		{name: "definitions in a subshell", make: func(count int) string {
			return "(\n" + strings.Repeat("f() { echo a; }\n", count) + ")\n"
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			small, large := test.make(100), test.make(200)
			// When
			once := allocations(t, func() { _, _ = ParseScript(small) })
			twice := allocations(t, func() { _, _ = ParseScript(large) })

			// Then
			t.Logf("100: %d allocations, 200: %d", once, twice)
			if float64(twice) > 2.5*float64(once) {
				t.Fatalf("twice the text took %d allocations against %d: the parse is not linear in it", twice, once)
			}
		})
	}
}
