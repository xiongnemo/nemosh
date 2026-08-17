package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The options that used to be refused by name. Every expectation was measured
// from GNU on the machine this was written on, and the comment says what was
// observed where it is not obvious.

func TestXargsOptions(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{
			name: "-n groups the items", args: []string{"-n2", "echo"},
			stdin: "a\nb\nc\n", want: "a b\nc\n",
		},
		{
			// The one that matters most: NUL separation is the only way a
			// filename with a blank in it survives, and splitting on whitespace
			// is how `xargs rm` comes to delete the wrong thing.
			name: "-0 splits on NUL", args: []string{"-0", "echo"},
			stdin: "a b\x00c\x00", want: "a b c\n",
		},
		{
			// Substituted wherever the placeholder appears, including inside a
			// larger word -- which is why this is `[x]` and not `[ x ]`.
			name: "-I runs once per item", args: []string{"-I{}", "echo", "[{}]"},
			stdin: "x\ny\n", want: "[x]\n[y]\n",
		},
		{
			name: "-r does nothing with no input", args: []string{"-r", "echo", "empty"},
			stdin: "", want: "",
		},
		{
			// Without -r, xargs runs the command once with no items. `xargs echo`
			// on empty input prints a blank line, which is GNU's behaviour.
			name: "without -r it runs once", args: []string{"echo"},
			stdin: "", want: "\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "xargs", test.args, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("xargs: %v", err)
			}
			if got != test.want {
				t.Fatalf("xargs %v = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestGrepOptions(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "-o prints the match only", args: []string{"-o", "b"}, stdin: "aXbXc\n", want: "b\n"},
		{name: "-c counts matching lines", args: []string{"-c", "o"}, stdin: "foo\nbar\n", want: "1\n"},
		{
			// A word match is not \b: the match must not sit next to a word
			// character, which is the definition that also works for a pattern
			// starting with punctuation.
			name: "-w matches whole words", args: []string{"-w", "foo"},
			stdin: "foo\nfoobar\n", want: "foo\n",
		},
		{name: "-x matches whole lines", args: []string{"-x", "foo"}, stdin: "foo\nfoobar\n", want: "foo\n"},
		{name: "-m stops after n matches", args: []string{"-m1", "a"}, stdin: "a\na\n", want: "a\n"},
		{
			// -F takes the pattern literally, so the dot is a dot.
			name: "-F is a fixed string", args: []string{"-F", "a.c"},
			stdin: "a.c\nabc\n", want: "a.c\n",
		},
		{
			// -o with -v prints nothing: there is no matched text on a line
			// selected for not matching, and GNU prints nothing rather than the
			// whole line.
			name: "-o and -v together print nothing", args: []string{"-o", "-v", "b"},
			stdin: "a\n", want: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, _ := runFilter(t, "grep", test.args, test.stdin)

			// Then
			if got != test.want {
				t.Fatalf("grep %v = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

// -q says nothing and answers with the status, which is the whole point.
func TestGrepQuiet(t *testing.T) {
	// When
	got, _, err := runFilter(t, "grep", []string{"-q", "foo"}, "foo\n")

	// Then
	if err != nil {
		t.Fatalf("grep -q: %v", err)
	}
	if got != "" {
		t.Fatalf("grep -q wrote %q, want nothing", got)
	}
}

// -r is the one that was missed most, and its output shape is the part that
// matters: the filename appears even when the tree held one file, because the
// operand was a directory and which file matched is what the reader does not
// know. Measured: `grep -r hit rd` prints `rd/f.txt:hit`.
func TestGrepRecursive(t *testing.T) {
	// Given
	directory := t.TempDir()
	nested := filepath.Join(directory, "nested")
	if err := os.Mkdir(nested, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "f.txt"), []byte("hit\nmiss\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// When
	got, err := runWithFiles(t, "grep", nil, "-r", "hit", directory)

	// Then
	if err != nil {
		t.Fatalf("grep -r: %v", err)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "f.txt:hit") {
		t.Fatalf("grep -r = %q, want the file named beside the match", got)
	}
}

// A directory named without -r is skipped with a diagnostic rather than read as
// bytes, which is what GNU does too.
func TestGrep_skipsADirectoryWithoutRecursion(t *testing.T) {
	// When
	got, err := runWithFiles(t, "grep", nil, "hit", t.TempDir())

	// Then
	if err == nil {
		t.Fatal("grep reported a match in a directory it did not read")
	}
	if got != "" {
		t.Fatalf("grep wrote %q to stdout, want nothing", got)
	}
}

func TestSortOptions(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{
			// The field separator without -t is the transition from blank to
			// non-blank, so an aligned column works. Splitting on a single space
			// would make `a   b` four fields and -k2 would compare nothing.
			name: "-k sorts by a field", args: []string{"-k2"},
			stdin: "b 2\na 3\nc 1\n", want: "c 1\nb 2\na 3\n",
		},
		{
			// -u removes duplicates *by the comparison key*, so folding case
			// makes B and b one line -- and the one that survives is the first
			// in sorted order.
			name: "-u with -f keeps one of an equal pair", args: []string{"-uf"},
			stdin: "B\nb\na\n", want: "a\nB\n",
		},
		{
			name: "-t chooses the separator", args: []string{"-t:", "-k1", "-n"},
			stdin: "3:a\n1:b\n", want: "1:b\n3:a\n",
		},
		{name: "-u alone", args: []string{"-u"}, stdin: "b\na\nb\n", want: "a\nb\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "sort", test.args, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("sort: %v", err)
			}
			if got != test.want {
				t.Fatalf("sort %v = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

// tail -c was deliberately absent while head -c existed, and the asymmetry was
// documented rather than fixed. This is the fix.
//
//	$ printf '0123456789' | tail -c 3   ->  789
//
// No newline is added: -c deals in bytes, and inventing one would change the
// length of what was asked for.
func TestTailBytes(t *testing.T) {
	// When
	got, _, err := runFilter(t, "tail", []string{"-c", "3"}, "0123456789")

	// Then
	if err != nil {
		t.Fatalf("tail -c: %v", err)
	}
	if got != "789" {
		t.Fatalf("tail -c 3 = %q, want %q", got, "789")
	}
}

func TestUniqOptions(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "-d keeps only what repeated", args: []string{"-d"}, stdin: "a\na\nb\n", want: "a\n"},
		{name: "-u keeps only what did not", args: []string{"-u"}, stdin: "a\na\nb\n", want: "b\n"},
		{name: "-i folds case into one run", args: []string{"-i"}, stdin: "A\na\n", want: "A\n"},
		{
			// Opposites, so both together select nothing. That falls out of
			// applying both conditions rather than being a special case, and it
			// is what GNU prints.
			name: "-d and -u together select nothing", args: []string{"-du"},
			stdin: "a\na\nb\n", want: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "uniq", test.args, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("uniq: %v", err)
			}
			if got != test.want {
				t.Fatalf("uniq %v = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

// -m counts characters where -c counts bytes, and -L is the longest line.
//
// GNU said 19 for both -c and -m at first, which is a locale artifact rather than
// a disagreement: with no locale set a character is a byte. Under LC_ALL=C.UTF-8
// it says 18, which is what this counts -- runes are what everything else here
// measures in.
func TestWcCharactersAndLongest(t *testing.T) {
	input := "héllo\nlonger line\n"
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "-c is bytes", args: []string{"-c"}, want: "19\n"},
		{name: "-m is characters", args: []string{"-m"}, want: "18\n"},
		{name: "-L is the longest line, without its newline", args: []string{"-L"}, want: "11\n"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "wc", test.args, input)

			// Then
			if err != nil {
				t.Fatalf("wc: %v", err)
			}
			if got != test.want {
				t.Fatalf("wc %v = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

// The default stays lines, words and bytes. Adding -m to it would change the
// output of every script that already parses wc.
func TestWc_defaultIsStillThreeCounts(t *testing.T) {
	// When
	got, _, err := runFilter(t, "wc", nil, "a b\n")

	// Then
	if err != nil {
		t.Fatalf("wc: %v", err)
	}
	if fields := strings.Fields(got); len(fields) != 3 {
		t.Fatalf("wc = %q, want three counts", got)
	}
}
