package runtime_test

import "testing"

// `shopt -s nocasematch` folds case where bash 5.3 folds it -- case, every [[ ]] comparison,
// and `${x/pattern/replacement}` -- and nowhere else: the `#` and `%` trims match as written.
// The transcript is bash's, measured; the option used to be refused.
func TestNoCaseMatch_foldsWhereBashDoes(t *testing.T) {
	script := "shopt -s nocasematch\n" +
		"case ABC in abc) echo case;; esac\n" +
		"[[ ABC == a* ]] && echo eq\n" +
		"[[ ABC != a* ]] || echo ne\n" +
		"[[ ABC =~ ^ab ]] && echo re\n" +
		"[[ \"ABC\" == \"abc\" ]] && echo quoted\n" +
		"x=ABC\necho \"${x/b/-} ${x#a} ${x%%c}\"\n" +
		"a=(XY xz)\necho \"${a[@]//x/_}\"\n" +
		"shopt -u nocasematch\n" +
		"case ABC in abc) echo leaked;; *) echo off;; esac\n"
	want := "case\neq\nne\nre\nquoted\nA-C ABC ABC\n_Y _z\noff\n"
	if stdout, _ := runScriptCapturing(script); stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
