package runtime_test

import "testing"

// A `;` inside an expansion in a brace group is the expansion's: `${s%;}` trims one, and
// `${x//;/-}` replaces each. The pass that turns a group's separators into newlines cut them
// there, and the lexer said "missing '}'". Both references print every line, measured.
func TestBraceGroup_aSemicolonInAnExpansionIsItsOwn(t *testing.T) {
	script := "f() {\n    local s=\"a;\"\n    s=${s%;}\n    s=${s#;}\n    echo \"[$s]\"\n}\nf\n" +
		"g() { x=\"b;c\"; echo ${x//;/-}; }; g\nh() { y=$(echo d; echo e); echo $y; }; h\n"
	want := "[a]\nb-c\nd e\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
