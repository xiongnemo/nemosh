package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Addresses, -n, -e, -E, and the p/d/q commands.
//
// sed was an `s///` filter and nothing else: no -n, no addresses, no d, no p.
// `sed -n '5,10p'` and `sed '/x/d'` are the two idioms people actually type, and
// neither ran. Measured against busybox-w32 v1.38.0 on 2026-08-22.

func runSedIn(t *testing.T, dir, stdin string, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("sed")
	if !ok {
		t.Fatal("sed is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := applet.Run(ctx, args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func sedFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "s.txt"), []byte("a1\nb2\nc3\nd4\ne5\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f1.txt"), []byte("a1\na2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "f2.txt"), []byte("b1\nb2\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSed_addressesAndCommands(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "quiet with a line address", args: []string{"-n", "2p", "s.txt"}, want: "b2\n"},
		{name: "a line range", args: []string{"-n", "2,4p", "s.txt"}, want: "b2\nc3\nd4\n"},
		{name: "delete one line", args: []string{"2d", "s.txt"}, want: "a1\nc3\nd4\ne5\n"},
		{name: "delete by pattern", args: []string{"/b/d", "s.txt"}, want: "a1\nc3\nd4\ne5\n"},
		{name: "a pattern range", args: []string{"-n", "/b/,/d/p", "s.txt"}, want: "b2\nc3\nd4\n"},
		{name: "the last line", args: []string{"-n", "$p", "s.txt"}, want: "e5\n"},
		{name: "to the last line", args: []string{"2,$d", "s.txt"}, want: "a1\n"},
		{name: "a negated address", args: []string{"-n", "2!p", "s.txt"}, want: "a1\nc3\nd4\ne5\n"},
		{name: "several scripts", args: []string{"-e", "1d", "-e", "3d", "s.txt"}, want: "b2\nd4\ne5\n"},
		{name: "an address on a substitution", args: []string{"1,2s/[0-9]/X/", "s.txt"}, want: "aX\nbX\nc3\nd4\ne5\n"},
		// Without -n, p prints a second copy: the line is written once because
		// it reached the end of the script and once because p asked.
		{name: "p without -n duplicates", args: []string{"p", "s.txt"}, want: "a1\na1\nb2\nb2\nc3\nc3\nd4\nd4\ne5\ne5\n"},
		{name: "semicolons separate commands", args: []string{"-n", "1p;3p", "s.txt"}, want: "a1\nc3\n"},
		{name: "two substitutions in one script", args: []string{"s/a/X/;s/1/Y/", "s.txt"}, want: "XY\nb2\nc3\nd4\ne5\n"},
		{name: "q stops after printing", args: []string{"2q", "s.txt"}, want: "a1\nb2\n"},
		{name: "extended regex with -E", args: []string{"-E", "s/(a|b)[0-9]/Z/", "s.txt"}, want: "Z\nZ\nc3\nd4\ne5\n"},
		{name: "-r is the same as -E", args: []string{"-r", "s/(a|b)[0-9]/Z/", "s.txt"}, want: "Z\nZ\nc3\nd4\ne5\n"},
		{name: "a newline separates commands too", args: []string{"-n", "1p\n3p", "s.txt"}, want: "a1\nc3\n"},
		{name: "an address range that never closes runs to the end", args: []string{"-n", "/c/,/nosuch/p", "s.txt"}, want: "c3\nd4\ne5\n"},
		// A range whose numeric end has already gone by is one line long. `$,1p`
		// opens on the last line and closes on it: measured, busybox answers e5.
		{name: "a range whose end has passed is one line", args: []string{"-n", "$,1p", "s.txt"}, want: "e5\n"},
		{name: "a pattern that matches nothing prints nothing", args: []string{"-n", "/unclosed/p", "s.txt"}, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Given
			dir := sedFixture(t)

			// When
			stdout, stderr, err := runSedIn(t, dir, "", test.args...)

			// Then
			if err != nil {
				t.Fatalf("sed %v: %v (stderr %q)", test.args, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sed %v\n  got  %q\n  want %q", test.args, stdout, test.want)
			}
		})
	}
}

// Several files are ONE stream: line numbers run on across the boundary and `$`
// is the last line of the last file. Measured -- `sed -n '3p' f1.txt f2.txt`
// answers b1, the third line overall.
//
// This is why the per-file loop had to go. It did not matter while sed had no
// addresses, and would have been silently wrong the moment it did.
func TestSed_severalFilesAreOneStream(t *testing.T) {
	dir := sedFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-n", "$p", "f1.txt", "f2.txt"}, want: "b2\n"},
		{args: []string{"-n", "1p", "f1.txt", "f2.txt"}, want: "a1\n"},
		{args: []string{"-n", "3p", "f1.txt", "f2.txt"}, want: "b1\n"},
		{args: []string{"-n", "2,3p", "f1.txt", "f2.txt"}, want: "a2\nb1\n"},
	} {
		stdout, stderr, err := runSedIn(t, dir, "", test.args...)
		if err != nil {
			t.Fatalf("sed %v: %v (%s)", test.args, err, stderr)
		}
		if stdout != test.want {
			t.Fatalf("sed %v = %q, want %q", test.args, stdout, test.want)
		}
	}
}

func TestSed_readsStdinWithAddresses(t *testing.T) {
	dir := sedFixture(t)
	stdout, _, err := runSedIn(t, dir, "one\ntwo\nthree\n", "-n", "2p")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "two\n" {
		t.Fatalf("sed -n 2p from stdin = %q, want two", stdout)
	}
	// `$` on a stream still needs to know which line is last, which means one
	// line of lookahead rather than reading the whole input.
	stdout, _, err = runSedIn(t, dir, "one\ntwo\nthree\n", "-n", "$p")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "three\n" {
		t.Fatalf("sed -n $p from stdin = %q, want three", stdout)
	}
}

// What sed still cannot do is refused by name, so a script asking for it fails
// rather than quietly getting something else.
func TestSed_refusesWhatItCannotDo(t *testing.T) {
	for _, test := range []struct {
		args     []string
		wantWord string
	}{
		{args: []string{"1a\\text", "s.txt"}, wantWord: "a"},
		{args: []string{"-n", "1~2p", "s.txt"}, wantWord: "~"},
		{args: []string{"-n", "1,2h", "s.txt"}, wantWord: "h"},
		// A word of nonsense is reported by its first unimplemented letter,
		// which is what busybox does too: it answers `unsupported command o`
		// here, `n` being a command it has and this does not.
		{args: []string{"nosuchcommand", "s.txt"}, wantWord: "n"},
	} {
		t.Run(strings.Join(test.args, " "), func(t *testing.T) {
			dir := sedFixture(t)
			stdout, stderr, err := runSedIn(t, dir, "", test.args...)
			if err == nil {
				t.Fatalf("sed %v succeeded, want a refusal", test.args)
			}
			if !strings.Contains(stderr+err.Error(), test.wantWord) {
				t.Fatalf("sed %v said %q, want it to name %q", test.args, stderr+err.Error(), test.wantWord)
			}
			if stdout != "" {
				t.Fatalf("sed %v wrote %q before refusing", test.args, stdout)
			}
		})
	}
}

// A bad address is refused before any line is read, for the same reason find
// validates its expression first: a caller acting on the output must not receive
// half an answer.
func TestSed_refusesABadAddress(t *testing.T) {
	dir := sedFixture(t)
	// `/unclosed/p` and `$,1p` were in this list and should not have been: the
	// first is a perfectly good script that happens to match nothing, and the
	// second is a range whose numeric end has already passed, which busybox
	// answers with the last line. Both are asserted as *working* above. Checked
	// against the reference rather than assumed.
	for _, script := range []string{"2,p", "/a", "1,2,3p"} {
		stdout, stderr, err := runSedIn(t, dir, "", "-n", script, "s.txt")
		if err == nil {
			t.Fatalf("sed -n %q succeeded, want a refusal", script)
		}
		if stdout != "" {
			t.Fatalf("sed -n %q wrote %q before refusing", script, stdout)
		}
		if stderr+err.Error() == "" {
			t.Fatalf("sed -n %q refused with no message", script)
		}
	}
}

// `s///i` folds case, which busybox has and this refused -- incoherently, since
// splitSedSubstituteFlags consumed the letter and parseSedSubstituteFlags then
// rejected it, so the two halves of one parser disagreed about which flags exist.
func TestSed_substituteIgnoresCase(t *testing.T) {
	dir := sedFixture(t)
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"s/A/X/i", "s.txt"}, want: "X1\nb2\nc3\nd4\ne5\n"},
		{args: []string{"s/A/X/I", "s.txt"}, want: "X1\nb2\nc3\nd4\ne5\n"},
		{args: []string{"s/[ABC]/X/gi", "s.txt"}, want: "X1\nX2\nX3\nd4\ne5\n"},
		// Without the flag the pattern stays case-sensitive.
		{args: []string{"s/A/X/", "s.txt"}, want: "a1\nb2\nc3\nd4\ne5\n"},
	} {
		stdout, stderr, err := runSedIn(t, dir, "", test.args...)
		if err != nil {
			t.Fatalf("sed %v: %v (%s)", test.args, err, stderr)
		}
		if stdout != test.want {
			t.Fatalf("sed %v = %q, want %q", test.args, stdout, test.want)
		}
	}
}

// An empty script is a valid no-op, not an error. Refusing it made
// `sed "$expr" file` fail whenever the variable was empty, where every reference
// passes the input through.
func TestSed_emptyScriptIsANoOp(t *testing.T) {
	dir := sedFixture(t)
	stdout, stderr, err := runSedIn(t, dir, "", "", "s.txt")
	if err != nil {
		t.Fatalf("sed '': %v (%s)", err, stderr)
	}
	if want := "a1\nb2\nc3\nd4\ne5\n"; stdout != want {
		t.Fatalf("sed '' = %q, want the file unchanged", stdout)
	}
	stdout, _, err = runSedIn(t, dir, "", "-n", "", "s.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "" {
		t.Fatalf("sed -n '' = %q, want nothing", stdout)
	}
}

// Line numbering starts at 1, so address 0 names no line. GNU refuses it; busybox
// lets it parse as *no address*, so `sed -n '0p'` prints every line, which is a
// quirk rather than a rule worth copying. The refusal has to name the address --
// it used to report `unsupported command 0` and blame the wrong thing.
func TestSed_refusesLineAddressZero(t *testing.T) {
	dir := sedFixture(t)
	stdout, stderr, err := runSedIn(t, dir, "", "-n", "0p", "s.txt")
	if err == nil {
		t.Fatal("sed -n 0p succeeded, want a refusal")
	}
	if stdout != "" {
		t.Fatalf("sed -n 0p wrote %q", stdout)
	}
	if message := stderr + err.Error(); !strings.Contains(message, "address 0") {
		t.Fatalf("sed -n 0p said %q, want it to name the address", message)
	}
}

// `{}` groups commands under one address, which is what makes `/x/{p;q}` apply
// both to the matching line and neither to any other. It also turns the walk over
// the commands into a recursive one, so `d` and `q` inside a block have to end the
// whole cycle rather than just the block.
func TestSed_blocks(t *testing.T) {
	dir := sedFixture(t)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "two commands under one address", args: []string{"-n", "/b/{p;p}", "s.txt"}, want: "b2\nb2\n"},
		{name: "substitutions in a block", args: []string{"2{s/b/X/;s/2/Y/}", "s.txt"}, want: "a1\nXY\nc3\nd4\ne5\n"},
		{name: "an address inside a range", args: []string{"-n", "/a/,/c/{/b/p}", "s.txt"}, want: "b2\n"},
		// d inside a block ends the cycle, so the automatic print does not happen.
		{name: "delete inside a block", args: []string{"2{d}", "s.txt"}, want: "a1\nc3\nd4\ne5\n"},
		// q inside a block stops the run, printing the line on the way out.
		{name: "quit inside a block", args: []string{"-n", "2{p;q}", "s.txt"}, want: "b2\n"},
		{name: "nested blocks", args: []string{"-n", "/a/,/c/{/b/{p}}", "s.txt"}, want: "b2\n"},
		{name: "a block with spaces", args: []string{"-n", "2 { p }", "s.txt"}, want: "b2\n"},
		{name: "a block with no address runs on every line", args: []string{"-n", "{p}", "s.txt"}, want: "a1\nb2\nc3\nd4\ne5\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runSedIn(t, dir, "", test.args...)
			if err != nil {
				t.Fatalf("sed %v: %v (%s)", test.args, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sed %v\n  got  %q\n  want %q", test.args, stdout, test.want)
			}
		})
	}
	// An unbalanced brace is named for which one it is.
	for _, script := range []string{"2{p", "2p}", "/a/{p;{q}"} {
		if _, _, err := runSedIn(t, dir, "", "-n", script, "s.txt"); err == nil {
			t.Fatalf("sed -n %q succeeded, want a refusal", script)
		}
	}
}

// y transliterates, character for character.
func TestSed_translate(t *testing.T) {
	dir := sedFixture(t)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "one for one", args: []string{"y/abc/xyz/", "s.txt"}, want: "x1\ny2\nz3\nd4\ne5\n"},
		{name: "with another command after it", args: []string{"y/ab/xy/;s/1/9/", "s.txt"}, want: "x9\ny2\nc3\nd4\ne5\n"},
		{name: "under an address", args: []string{"2y/b/X/", "s.txt"}, want: "a1\nX2\nc3\nd4\ne5\n"},
		{name: "another delimiter", args: []string{"y,ab,xy,", "s.txt"}, want: "x1\ny2\nc3\nd4\ne5\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runSedIn(t, dir, "", test.args...)
			if err != nil {
				t.Fatalf("sed %v: %v (%s)", test.args, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sed %v = %q, want %q", test.args, stdout, test.want)
			}
		})
	}

	// Runes, not bytes: transliterating by byte would replace half a character
	// and corrupt the output.
	stdout, _, err := runSedIn(t, dir, "\u00e1\u00e9\n", "y/\u00e1\u00e9/ae/")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "ae\n" {
		t.Fatalf("y over multibyte input = %q, want ae", stdout)
	}

	// Unequal lengths are refused. busybox transliterates the pairs it has and
	// silently ignores the rest -- `y/abc/xy/` leaves every c alone, measured --
	// which is a wrong answer with no diagnostic. GNU refuses it, and so does this.
	_, stderr, err := runSedIn(t, dir, "", "y/abc/xy/", "s.txt")
	if err == nil {
		t.Fatal("sed y with unequal lengths succeeded, want a refusal")
	}
	if !strings.Contains(stderr+err.Error(), "different lengths") {
		t.Fatalf("sed y with unequal lengths said %q, want it to name the cause", stderr+err.Error())
	}
}

// = writes the line number on a line of its own, before the line.
func TestSed_lineNumber(t *testing.T) {
	dir := sedFixture(t)
	stdout, stderr, err := runSedIn(t, dir, "", "-n", "=", "s.txt")
	if err != nil {
		t.Fatalf("sed -n =: %v (%s)", err, stderr)
	}
	if want := "1\n2\n3\n4\n5\n"; stdout != want {
		t.Fatalf("sed -n = %q, want %q", stdout, want)
	}
	stdout, _, err = runSedIn(t, dir, "", "-n", "2=", "s.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "2\n" {
		t.Fatalf("sed -n 2= = %q, want 2", stdout)
	}
	// Without -n the number precedes the line, which is what makes `sed =` a
	// crude `nl`.
	stdout, _, err = runSedIn(t, dir, "", "=", "s.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\na1\n2\nb2\n3\nc3\n4\nd4\n5\ne5\n"; stdout != want {
		t.Fatalf("sed = %q, want %q", stdout, want)
	}
}

// -f takes the script from a file, whose lines are commands exactly as a
// `;`-separated script's are.
func TestSed_scriptFile(t *testing.T) {
	dir := sedFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "script.sed"), []byte("1d\n3d\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runSedIn(t, dir, "", "-f", "script.sed", "s.txt")
	if err != nil {
		t.Fatalf("sed -f: %v (%s)", err, stderr)
	}
	if want := "b2\nd4\ne5\n"; stdout != want {
		t.Fatalf("sed -f = %q, want %q", stdout, want)
	}

	// -f and -e combine, in the order given.
	stdout, _, err = runSedIn(t, dir, "", "-f", "script.sed", "-e", "s/b/X/", "s.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "X2\nd4\ne5\n"; stdout != want {
		t.Fatalf("sed -f -e = %q, want %q", stdout, want)
	}

	// A missing script file is an error about that file, not about a script.
	_, stderr, err = runSedIn(t, dir, "", "-f", "nosuch.sed", "s.txt")
	if err == nil {
		t.Fatal("sed -f with a missing file succeeded")
	}
	if !strings.Contains(stderr+err.Error(), "nosuch.sed") {
		t.Fatalf("sed -f said %q, want it to name the file", stderr+err.Error())
	}
}

// -i edits in place. Each file is its own stream: line numbers restart, `$` is
// that file's last line, and an address range does not leak into the next file.
//
// That last part is GNU's behaviour and the coherent reading of what -i means.
// busybox restarts the numbering but leaves an open range running across the
// boundary, so `sed -i -n '2,3p' a b` keeps b's first line because a's range
// never closed -- measured 2026-08-22, and state reuse rather than a rule.
func TestSed_inPlace(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	read := func(name string) string {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	// The plain form writes nothing to stdout.
	write("t.txt", "a1\nb2\nc3\n")
	stdout, stderr, err := runSedIn(t, dir, "", "-i", "s/a/X/", "t.txt")
	if err != nil {
		t.Fatalf("sed -i: %v (%s)", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("sed -i wrote %q to stdout", stdout)
	}
	if got := read("t.txt"); got != "X1\nb2\nc3\n" {
		t.Fatalf("sed -i left %q", got)
	}

	// -i.bak keeps the original under that suffix.
	write("u.txt", "a1\nb2\n")
	if _, _, err := runSedIn(t, dir, "", "-i.bak", "s/a/X/", "u.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read("u.txt"); got != "X1\nb2\n" {
		t.Fatalf("sed -i.bak left %q", got)
	}
	if got := read("u.txt.bak"); got != "a1\nb2\n" {
		t.Fatalf("backup holds %q, want the original", got)
	}

	// Each file is its own stream, so `$d` drops the last line of both.
	write("f1.txt", "a1\na2\n")
	write("f2.txt", "b1\nb2\n")
	if _, _, err := runSedIn(t, dir, "", "-i", "$d", "f1.txt", "f2.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read("f1.txt"); got != "a1\n" {
		t.Fatalf("f1 = %q, want its last line dropped", got)
	}
	if got := read("f2.txt"); got != "b1\n" {
		t.Fatalf("f2 = %q, want its last line dropped", got)
	}

	// And a range does not leak: f2 keeps only its own line 2.
	write("h1.txt", "a1\na2\n")
	write("h2.txt", "b1\nb2\n")
	if _, _, err := runSedIn(t, dir, "", "-i", "-n", "2,3p", "h1.txt", "h2.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read("h1.txt"); got != "a2\n" {
		t.Fatalf("h1 = %q, want a2", got)
	}
	if got := read("h2.txt"); got != "b2\n" {
		t.Fatalf("h2 = %q, want b2 -- a range leaked from the previous file", got)
	}

	// A missing operand is reported and the others still edited.
	write("ok.txt", "a1\n")
	_, stderr, err = runSedIn(t, dir, "", "-i", "s/a/X/", "nosuch.txt", "ok.txt")
	if err == nil {
		t.Fatal("sed -i on a missing file succeeded, want status 1")
	}
	if !strings.Contains(stderr, "nosuch.txt") {
		t.Fatalf("stderr = %q, want it to name the missing file", stderr)
	}
	if got := read("ok.txt"); got != "X1\n" {
		t.Fatalf("ok.txt = %q, want it edited despite the earlier failure", got)
	}

	// There is nothing to edit in place when the input is a stream.
	if _, _, err := runSedIn(t, dir, "a\n", "-i", "s/a/X/"); err == nil {
		t.Fatal("sed -i with no operands succeeded, want a refusal")
	}

	// A failing script leaves the file alone rather than truncating it.
	write("keep.txt", "a1\n")
	if _, _, err := runSedIn(t, dir, "", "-i", "s/a/X/", "keep.txt"); err != nil {
		t.Fatal(err)
	}
	if got := read("keep.txt"); got != "X1\n" {
		t.Fatalf("keep.txt = %q", got)
	}
}
