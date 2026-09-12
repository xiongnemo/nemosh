package runtime

import "testing"

// The name is split at the *last* character being a bracket, so a word that merely
// contains one is a name. Otherwise `unset a[1]x` would remove something nobody named.
func TestSplitSubscriptedName(t *testing.T) {
	for _, test := range []struct {
		text      string
		name      string
		subscript string
		ok        bool
	}{
		{text: "a[1]", name: "a", subscript: "1", ok: true},
		{text: "a[i+1]", name: "a", subscript: "i+1", ok: true},
		{text: "m[a key]", name: "m", subscript: "a key", ok: true},
		{text: "a[@]", name: "a", subscript: "@", ok: true},
		// Not subscripts.
		{text: "a", name: "a", subscript: "", ok: false},
		{text: "a[1]x", name: "a[1]x", subscript: "", ok: false},
		{text: "[1]", name: "[1]", subscript: "", ok: false},
		{text: "", name: "", subscript: "", ok: false},
	} {
		t.Run(test.text, func(t *testing.T) {
			name, subscript, ok := splitSubscriptedName(test.text)
			if name != test.name || subscript != test.subscript || ok != test.ok {
				t.Fatalf("splitSubscriptedName(%q) = %q, %q, %v; want %q, %q, %v",
					test.text, name, subscript, ok, test.name, test.subscript, test.ok)
			}
		})
	}
}
