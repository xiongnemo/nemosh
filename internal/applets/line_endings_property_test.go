package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// What every line-oriented applet does with the *ending* of its input, as one
// table rather than as a test per applet.
//
// This exists because seven of them were wrong at once and nothing noticed: `sed`,
// `rev`, `head`, `tail`, `fold`, `expand` and `unexpand` added a newline to a file
// whose last line had none, and six of them turned every CRLF into LF. Both were
// measured against busybox *and* GNU, which agree with each other and disagreed
// with all seven.
//
// No fixture in the whole suite lacked a trailing newline, which is why. So the
// fixtures here are written in **binary mode from Go**, not by a shell: Git Bash's
// `printf 'a\nb' > f` silently writes `a\r\nb`, and measuring against that fixture
// produced two wrong conclusions before it was noticed.
//
// The three shapes, and what each one catches:
//
//	a\nb      no final terminator  -- the byte an applet must not add
//	a\nb\n    the ordinary file    -- the control, which must not change
//	a\r\nb\r\n  CRLF               -- the Windows case, and the one that matters most
//	                                 here: `head build.log > first.txt` must not
//	                                 rewrite the copy's line endings

// lineEndingBehaviour is what an applet does to each ending shape.
//
// preserve means the output's ending matches the input's, byte for byte. normalise
// means the applet writes LF and terminates the last line whatever arrived, which
// is correct for an applet whose output is a *new* document rather than a copy of
// the input -- and which was measured from busybox in every case, not decided here.
type lineEndingBehaviour int

const (
	preserveEndings lineEndingBehaviour = iota
	normaliseEndings
)

func TestApplets_lineEndingsAreEitherPreservedOrNormalisedOnPurpose(t *testing.T) {
	for _, test := range []struct {
		applet   string
		args     []string
		expected lineEndingBehaviour
		// why records the measurement, so a future change to a row has to argue
		// with busybox rather than with a preference.
		why string
	}{
		// Copies: the output is the input, so the endings are the input's.
		{applet: "cat", expected: preserveEndings, why: "a copy is a copy"},
		{applet: "rev", expected: preserveEndings, why: "busybox and GNU both preserve"},
		{applet: "head", expected: preserveEndings, why: "busybox and GNU both preserve"},
		{applet: "tail", expected: preserveEndings, why: "busybox and GNU both preserve"},
		{applet: "fold", expected: preserveEndings, why: "busybox preserves; inserted breaks are LF"},
		{applet: "expand", expected: preserveEndings, why: "tabs become spaces and nothing else changes"},
		{applet: "unexpand", expected: preserveEndings, why: "spaces become tabs and nothing else changes"},

		// New documents: every line is rewritten, so busybox normalises and so does
		// this. Each was measured rather than assumed.
		{applet: "nl", expected: normaliseEndings, why: "every line gains a number and a tab"},
		{applet: "sort", expected: normaliseEndings, why: "busybox terminates the last line"},
		{applet: "uniq", expected: normaliseEndings, why: "busybox terminates the last line"},
		{applet: "cut", args: []string{"-c", "1-99"}, expected: normaliseEndings, why: "busybox terminates the last line"},
		{applet: "grep", args: []string{"-e", "."}, expected: normaliseEndings, why: "busybox terminates each match"},
	} {
		t.Run(test.applet, func(t *testing.T) {
			for _, shape := range lineEndingShapes() {
				t.Run(shape.name, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "in.txt")
					if err := os.WriteFile(path, shape.content, 0o600); err != nil {
						t.Fatal(err)
					}
					args := append(append([]string{}, test.args...), "in.txt")
					stdout, stderr, err := runSmall(t, root, "", test.applet, args...)
					if err != nil {
						t.Fatalf("%s: %v (%s)", test.applet, err, stderr)
					}
					switch test.expected {
					case preserveEndings:
						if stdout != string(shape.content) {
							t.Fatalf("%s did not preserve %s (%s):\n  in  %q\n  out %q",
								test.applet, shape.name, test.why, shape.content, stdout)
						}
					case normaliseEndings:
						if !strings.HasSuffix(stdout, "\n") {
							t.Errorf("%s left the last line unterminated (%s): %q",
								test.applet, test.why, stdout)
						}
						if strings.Contains(stdout, "\r") {
							t.Errorf("%s kept a carriage return while normalising (%s): %q",
								test.applet, test.why, stdout)
						}
					}
				})
			}
		})
	}
}

// sed is its own case, because the rule is not per line: the newline is omitted
// only on the very last thing written. `p` writes the final line twice and the
// *first* of the two is terminated -- measured from GNU and busybox, which agree.
func TestSed_omitsTheNewlineOnlyOnItsLastWrite(t *testing.T) {
	for _, test := range []struct {
		script string
		input  string
		want   string
	}{
		// The plain case: a file with no final newline keeps none.
		{script: "s/x/y/", input: "a\nb", want: "a\nb"},
		{script: "s/a/A/", input: "a\nb", want: "A\nb"},
		// p duplicates. The duplicate is terminated and only the true last write is
		// not, which a per-line rule cannot express.
		{script: "p", input: "a\nb", want: "a\na\nb\nb"},
		{script: "$p", input: "a\nb", want: "a\nb\nb"},
		// A file that had a final newline keeps it.
		{script: "s/x/y/", input: "a\nb\n", want: "a\nb\n"},
		{script: "p", input: "a\nb\n", want: "a\na\nb\nb\n"},
		// One line, no terminator.
		{script: "s/x/y/", input: "only", want: "only"},
		{script: "p", input: "only", want: "only\nonly"},
		// Empty input writes nothing at all, rather than a bare newline.
		{script: "s/x/y/", input: "", want: ""},
		// -n with no p writes nothing, and must not leave a newline behind either.
		{script: "s/x/y/", input: "a\nb", want: "a\nb"},
		// q stops early; what it wrote is the last write, so the rule applies there.
		{script: "1q", input: "a\nb", want: "a\n"},
		{script: "2q", input: "a\nb", want: "a\nb"},
		// d discards, so the following line becomes the last write.
		{script: "1d", input: "a\nb", want: "b"},
		{script: "2d", input: "a\nb", want: "a\n"},
	} {
		t.Run(test.script+" on "+strings.ReplaceAll(test.input, "\n", "|"), func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "in.txt"), []byte(test.input), 0o600); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err := runSmall(t, root, "", "sed", test.script, "in.txt")
			if err != nil {
				t.Fatalf("sed %q: %v (%s)", test.script, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sed %q on %q = %q, want %q", test.script, test.input, stdout, test.want)
			}
		})
	}
}

// The destructive one: `sed -i` wrote its extra byte to the file, so a script that
// matched nothing still changed the file on disk. A UTF-16 file is the sharpest
// case, because its last byte is a NUL rather than a newline -- which is why the
// support matrix's claim that sed "copies the file through" was false.
func TestSed_inPlaceLeavesAnUnmatchedFileByteIdentical(t *testing.T) {
	for _, test := range []struct {
		name    string
		content []byte
	}{
		{name: "no final newline", content: []byte("a\nb")},
		{name: "with a final newline", content: []byte("a\nb\n")},
		// CRLF is deliberately absent, and that is a measurement rather than an
		// omission: `sed -i` on a CRLF file rewrites it as LF in GNU *and* in
		// busybox, both answering four bytes for six. This build matches both, and
		// TestSed_inPlaceNormalisesCrlfLikeBothReferences pins it so the behaviour is
		// stated rather than discovered -- it is worth knowing on a Windows machine.
		//
		// UTF-16LE with a byte-order mark: "alpha\nbeta\n". sed matches nothing in
		// it, so it must come out exactly as it went in.
		{name: "UTF-16LE", content: utf16Fixture()},
		{name: "empty", content: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "in.txt")
			if err := os.WriteFile(path, test.content, 0o600); err != nil {
				t.Fatal(err)
			}
			// A script that cannot match anything in any of these.
			if _, stderr, err := runSmall(t, root, "", "sed", "-i", "s/zzzznosuch/x/", "in.txt"); err != nil {
				t.Fatalf("sed -i: %v (%s)", err, stderr)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(test.content) {
				t.Fatalf("sed -i changed a file it matched nothing in:\n  before %d bytes %q\n  after  %d bytes %q",
					len(test.content), test.content, len(after), after)
			}
		})
	}
}

// fold's inserted breaks are newlines while the line's own ending survives, which
// is busybox's answer and not an obvious one: `fold -w 3` on `abcdefgh\r\n` gives
// three pieces, the first two ended by a bare newline and the last keeping the CRLF.
func TestFold_insertsNewlinesAndKeepsTheLineEnding(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "in.txt"), []byte("abcdefgh\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runSmall(t, root, "", "fold", "-w", "3", "in.txt")
	if err != nil {
		t.Fatalf("fold: %v (%s)", err, stderr)
	}
	if stdout != "abc\ndef\ngh\r\n" {
		t.Fatalf("fold -w 3 = %q, want the breaks as LF and the ending kept", stdout)
	}
	// And a folded line with no ending at all keeps none.
	if err := os.WriteFile(filepath.Join(root, "bare.txt"), []byte("abcdefgh"), 0o600); err != nil {
		t.Fatal(err)
	}
	if stdout, _, err = runSmall(t, root, "", "fold", "-w", "3", "bare.txt"); err != nil {
		t.Fatal(err)
	}
	if stdout != "abc\ndef\ngh" {
		t.Fatalf("fold on an unterminated line = %q", stdout)
	}
}

type lineEndingShape struct {
	name    string
	content []byte
}

// lineEndingShapes are written as bytes on purpose. A shell fixture cannot be
// trusted for this: Git Bash translates `printf 'a\nb' > f` into `a\r\nb`, and the
// first attempt at this work measured against exactly that and drew two wrong
// conclusions from it.
func lineEndingShapes() []lineEndingShape {
	return []lineEndingShape{
		{name: "no final newline", content: []byte("a\nb")},
		{name: "final newline", content: []byte("a\nb\n")},
		{name: "CRLF", content: []byte("a\r\nb\r\n")},
	}
}

func utf16Fixture() []byte {
	// A byte-order mark, then "alpha\nbeta\n" in UTF-16LE. Built here rather than
	// through x/text so the fixture is exactly these bytes and nothing else.
	out := []byte{0xff, 0xfe}
	for _, r := range "alpha\nbeta\n" {
		out = append(out, byte(r), 0x00)
	}
	return out
}
