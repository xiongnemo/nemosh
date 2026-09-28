package runtime_test

import "testing"

// An elif chain may stand after other words and have words after its fi: `true && if ...`, `fi >
// /dev/null`, `fi | cat`, `fi | while read x`. The pass that nests each elif as an if in an
// else counted only a line that was exactly an opener or a closer. It paid the chain's extra fi
// to the wrong compound after `&&`, "duplicate then", and never paid it when the fi had a
// redirection or a pipe after it, "missing fi". Both references print every line, measured.
func TestElif_aChainTakesWordsOnEitherSide(t *testing.T) {
	script := "true && if false; then :; elif true; then echo x; fi\n" +
		"if false; then :; elif true; then echo y; fi > /dev/null; echo \"st=$?\"\n" +
		"if false; then :; elif true; then echo z; fi | cat\n" +
		"if false; then :\nelif true; then echo w\nfi | while read x; do echo \"r $x\"; done\n" +
		"if false; then :; elif false; then :; elif true; then echo v; fi 2>/dev/null | cat\n"
	want := "x\nst=0\nz\nr w\nv\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
