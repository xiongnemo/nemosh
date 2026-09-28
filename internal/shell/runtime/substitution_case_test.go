package runtime_test

import "testing"

// A command substitution in a case's arm, inside another substitution, ends at its own `)`: the
// case check took that `)` for a pattern's, so the scan for the outer one's end ran on to the
// end of the script, "unterminated command substitution". A case inside the inner one still
// has patterns of its own. Both references print every line, measured.
func TestCommandSubstitution_oneInACaseArmEndsAtItsOwnParenthesis(t *testing.T) {
	script := "i=a\nx=$(\n  case $i in\n  a) y=$(echo b); echo \"$y\";;\n  esac\n)\necho \"[$x]\"\n" +
		"z=$(case a in (a) w=$(case b in b) echo c;; esac); echo \"$w\";; esac)\necho \"[$z]\"\n"
	want := "[b]\n[c]\n"
	if stdout, status := runScriptCapturing(script); stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0, as both references answer", stdout, status, want)
	}
}
