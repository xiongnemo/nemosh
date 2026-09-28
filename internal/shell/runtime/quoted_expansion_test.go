package runtime_test

import "testing"

// Inside double quotes a `$(...)` or a `${...}` holds quotes of its own, and what they quote
// is no operator: `"$(echo "a ; b")"` is one word, and so is `"${u:-"c ; d"}"`. The scans that
// cut a line took the inner `"` for the outer one's close, so the `;` ended the command, the `)`
// closed a group that was never open, and each such line was refused. Both references print
// every line, measured.
func TestDoubleQuotes_anExpansionInsideHoldsItsOwnQuotes(t *testing.T) {
	script := "echo \"$(echo \"a ; b\")\"\n" +
		"echo \"$(echo \"a ) b\")\" \"$(echo \"a ( b\")\"\n" +
		"echo \"${u:-\"c ; d\"}\" \"${u:-$(echo \"c ) d\")}\"\n" +
		"x=1; echo \"${x:+\"e ; f\"}\"\n" +
		"if true; then echo \"$(echo \"g ; h\")\"; fi\n" +
		"case a in a) echo \"${u:-\"i ;; j\"}\";; esac\n" +
		"f() { echo \"$(echo \"k ; l\")\"; }; f\n" +
		"echo \"$(echo \"m & n\")\" & wait\n" +
		"{ echo \"$(echo \"o } p\")\"; }\n" +
		"echo \"$(echo \"q | r\")\" | cat\n" +
		"echo \"${u:-\"s ( t\"}\" \"${u:-\"s # t\"}\"\n" +
		"x=\"$(printf \"Usage: %s (options)\" me)\"; echo \"$x\"\n" +
		"echo \"$(echo \"u\nv ; w\")\"\n"
	want := "a ; b\na ) b a ( b\nc ; d c ) d\ne ; f\ng ; h\ni ;; j\nk ; l\nm & n\no } p\nq | r\n" +
		"s ( t s # t\nUsage: me (options)\nu\nv ; w\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
