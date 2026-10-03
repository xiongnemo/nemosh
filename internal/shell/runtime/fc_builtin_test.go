package runtime_test

import (
	"strings"
	"testing"
)

// fc is bash's: busybox has none, and the name ran Windows' fc.exe. Every expectation is bash
// 5.3's, measured on the same three entries, without bash's `line N:` prefix.
func TestFc_listsAsBashsDoes(t *testing.T) {
	load := `history -s "echo one"; history -s "echo two"; history -s "printf %s\\n three"` + "\n"
	all := "1\t echo one\n2\t echo two\n3\t printf %s\\n three\n"
	tests := []struct {
		name, script, want, fragment string
	}{
		{name: "the newest sixteen", script: load + "fc -l\n", want: all},
		{name: "without numbers", script: load + "fc -ln\n", want: "\t echo one\n\t echo two\n\t printf %s\\n three\n"},
		{name: "newest first", script: load + "fc -lr\n", want: "3\t printf %s\\n three\n2\t echo two\n1\t echo one\n"},
		{name: "from a number", script: load + "fc -l 2\n", want: "2\t echo two\n3\t printf %s\\n three\n"},
		{name: "from one counted back", script: load + "fc -l -2\n", want: "2\t echo two\n3\t printf %s\\n three\n"},
		{name: "a range", script: load + "fc -l 1 2\n", want: "1\t echo one\n2\t echo two\n"},
		// bash's range check takes the newest entry, named as FIRST, for one past the end.
		{name: "a FIRST that is the newest", script: load + "fc -l 3 1\n", want: "1\t echo one\n"},
		{name: "from the newest that starts with a word", script: load + "fc -l echo\n", want: "2\t echo two\n3\t printf %s\\n three\n"},
		{name: "a number past the end", script: load + "fc -l 99\n", want: all},
		{name: "one counted back past the start", script: load + "fc -l -99\n", want: all},
		{name: "0 is the newest", script: load + "fc -l 0\n", want: "3\t printf %s\\n three\n"},
		{name: "a word nothing starts with", script: load + "fc -l nomatch; echo s=$?\n", want: "s=1\n", fragment: "nemosh: line 2: fc: no command found"},
		{name: "an empty list", script: "fc -l; echo s=$?\n", want: "s=0\n"},
		{name: "an option it does not take", script: "fc -x; echo s=$?\n", want: "s=2\n", fragment: "fc: -x: invalid option\nfc: usage: "},
		{name: "-e with no editor", script: "fc -e; echo s=$?\n", want: "s=2\n", fragment: "fc: -e: option requires an argument"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			_, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.want {
				t.Fatalf("%q printed %q, want %q; stderr %q", test.script, stdout, test.want, stderr)
			}
			if !strings.Contains(stderr, test.fragment) || (test.fragment == "" && stderr != "") {
				t.Fatalf("stderr = %q, want %q", stderr, test.fragment)
			}
		})
	}
}

// fc -s runs an entry again, its pat=rep replacements made, says it on standard error, and
// puts it in the newest entry's place, as bash's fc_replhist does; fc -e - is the same. The
// printf entry runs again unquoted, so its format loses its backslash, as bash's does.
func TestFc_runsAnEntryAgain(t *testing.T) {
	script := `history -s "echo one"; history -s "echo two"; history -s "printf %s\\n three"` + "\n" +
		"fc -s; echo s=$?\n" +
		"fc -s two=2 echo; echo s=$?\n" +
		"fc -s nomatch; echo s=$?\n" +
		"history\n" +
		"fc -e - one=ONE 1; echo s=$?\n" +
		"history\n"

	// When
	_, stdout, stderr := runSetScript(t, script)

	// Then
	want := "threens=0\n2\ns=0\ns=1\n" + "1  echo one\n2  echo two\n3  echo 2\n" + "ONE\ns=0\n" + "1  echo one\n2  echo two\n3  echo ONE\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if wantErr := "printf %s\\n three\necho 2\nnemosh: line 4: fc: no command found\necho ONE\n"; stderr != wantErr {
		t.Fatalf("stderr = %q, want %q", stderr, wantErr)
	}
}

// Without -l, fc hands the entries to an editor, -e's or FCEDIT's, and runs what the file
// holds after: said on standard error first, and only when the editor succeeded. FCEDIT
// before fc is fc's alone. The first three are bash 5.3's, measured. By the fourth bash's
// list holds the script's own lines, its fc having turned recording on in a script, where
// this one's holds what history -s and fc put in it.
func TestFc_editsAndRuns(t *testing.T) {
	script := `history -s "echo one"; history -s "echo two"` + "\n" +
		"FCEDIT='sed -i s/one/ONE/' fc 1; echo s=$?\n" +
		"FCEDIT=false fc 1; echo s=$?\n" +
		`echo "FCEDIT=${FCEDIT-unset}"` + "\n" +
		"fc -r -e cat 1 2; echo s=$?\n"

	// When
	_, stdout, stderr := runSetScript(t, script)

	// Then
	want := "ONE\ns=0\n" + "s=1\n" + "FCEDIT=unset\n" + "echo two\necho one\ntwo\none\ns=0\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q; stderr %q", stdout, want, stderr)
	}
	if wantErr := "echo ONE\necho two\necho one\n"; stderr != wantErr {
		t.Fatalf("stderr = %q, want %q", stderr, wantErr)
	}
}
