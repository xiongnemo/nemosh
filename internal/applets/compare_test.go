package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// runWithFiles runs an applet in a directory of files and returns what it wrote.
func runWithFiles(t *testing.T, name string, files map[string]string, args ...string) (string, error) {
	t.Helper()
	directory := t.TempDir()
	for base, body := range files {
		if err := os.WriteFile(filepath.Join(directory, base), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	resolved := make([]string, len(args))
	for index, arg := range args {
		if _, isFile := files[arg]; isFile {
			resolved[index] = filepath.Join(directory, arg)
			continue
		}
		resolved[index] = arg
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), resolved, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), err
}

// cmp's message is GNU's, and it goes to stdout rather than stderr -- surprising,
// and measured:
//
//	$ cmp c1 c2
//	c1 c2 differ: char 3, line 1
//	$ echo $?
//	1
func TestCmp(t *testing.T) {
	files := map[string]string{"c1": "abc\n", "c2": "abd\n", "c3": "abc\n", "long": "abc\nmore\n"}

	t.Run("identical files say nothing and succeed", func(t *testing.T) {
		// When
		got, err := runWithFiles(t, "cmp", files, "c1", "c3")

		// Then
		if err != nil || got != "" {
			t.Fatalf("cmp = (%q, %v), want silence and success", got, err)
		}
	})

	t.Run("a difference names the byte and the line", func(t *testing.T) {
		// When
		got, err := runWithFiles(t, "cmp", files, "c1", "c2")

		// Then
		if err == nil {
			t.Fatal("cmp reported success for differing files")
		}
		if !strings.Contains(got, "differ: char 3, line 1") {
			t.Fatalf("cmp said %q, want GNU's wording", got)
		}
	})

	t.Run("-s says nothing and leaves only the status", func(t *testing.T) {
		// When
		got, err := runWithFiles(t, "cmp", files, "-s", "c1", "c2")

		// Then
		if err == nil {
			t.Fatal("cmp -s reported success for differing files")
		}
		if got != "" {
			t.Fatalf("cmp -s wrote %q, want nothing", got)
		}
	})

	t.Run("a prefix is reported as EOF rather than a byte that is not there", func(t *testing.T) {
		// When
		got, err := runWithFiles(t, "cmp", files, "c1", "long")

		// Then
		if err == nil {
			t.Fatal("cmp reported success for files of different length")
		}
		if !strings.Contains(got, "EOF on") {
			t.Fatalf("cmp said %q, want it to name the shorter file", got)
		}
	})
}

// comm's three columns are identified by how far they are indented, which is why
// suppressing one shifts the others left.
//
//	$ comm s1 s2      with s1 = a,b,c and s2 = b,c,d
//	a
//	\t\tb
//	\t\tc
//	\td
func TestComm(t *testing.T) {
	files := map[string]string{"s1": "a\nb\nc\n", "s2": "b\nc\nd\n"}

	t.Run("three columns", func(t *testing.T) {
		// When
		got, err := runWithFiles(t, "comm", files, "s1", "s2")

		// Then
		if err != nil {
			t.Fatalf("comm: %v", err)
		}
		if got != "a\n\t\tb\n\t\tc\n\td\n" {
			t.Fatalf("comm = %q, want GNU's three columns", got)
		}
	})

	t.Run("-12 leaves only what is in both, unindented", func(t *testing.T) {
		// Suppressing the first two columns removes two levels of indentation
		// from the third, which is why the output is bare.
		// When
		got, err := runWithFiles(t, "comm", files, "-12", "s1", "s2")

		// Then
		if err != nil {
			t.Fatalf("comm: %v", err)
		}
		if got != "b\nc\n" {
			t.Fatalf("comm -12 = %q, want the common lines with no indent", got)
		}
	})

	t.Run("-3 drops the common lines", func(t *testing.T) {
		// When
		got, err := runWithFiles(t, "comm", files, "-3", "s1", "s2")

		// Then
		if err != nil {
			t.Fatalf("comm: %v", err)
		}
		if got != "a\n\td\n" {
			t.Fatalf("comm -3 = %q", got)
		}
	})
}

func TestPaste(t *testing.T) {
	files := map[string]string{"p1": "1\n2\n", "p2": "a\nb\n", "p3": "x\n"}

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "tab by default", args: []string{"p1", "p2"}, want: "1\ta\n2\tb\n"},
		{name: "-d chooses the delimiter", args: []string{"-d,", "p1", "p2"}, want: "1,a\n2,b\n"},
		{
			// -s is the different one: each file's lines on one line of its own,
			// rather than the files read in parallel.
			name: "-s puts each file on one line", args: []string{"-s", "p1"}, want: "1\t2\n",
		},
		{
			// The shorter file contributes an empty field rather than ending the
			// output, which is what keeps the columns aligned.
			name: "a short file pads", args: []string{"p1", "p3"}, want: "1\tx\n2\t\n",
		},
		{
			// The delimiter list cycles, so `-d,;` alternates between columns.
			name: "the delimiter list cycles", args: []string{"-d,;", "p1", "p2", "p3"},
			want: "1,a;x\n2,b;\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, err := runWithFiles(t, "paste", files, test.args...)

			// Then
			if err != nil {
				t.Fatalf("paste: %v", err)
			}
			if got != test.want {
				t.Fatalf("paste %v = %q, want %q", test.args, got, test.want)
			}
		})
	}
}

func TestXxd(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{
			name: "the layout, to the column", stdin: "hello world",
			want: "00000000: 6865 6c6c 6f20 776f 726c 64              hello world\n",
		},
		{
			// Exactly sixteen bytes fill a line, and the seventeenth starts the
			// next one -- the boundary an off-by-one lands on.
			name: "a full line and one more", stdin: "0123456789abcdef0",
			want: "00000000: 3031 3233 3435 3637 3839 6162 6364 6566  0123456789abcdef\n" +
				"00000010: 30                                       0\n",
		},
		{
			// A byte that would move the cursor becomes a dot, or the dump would
			// emit control characters into the terminal it is being read in.
			name: "unprintable bytes become dots", stdin: "a\x00\x1bb",
			want: "00000000: 6100 1b62                                a..b\n",
		},
		{name: "-p is hex and nothing else", args: []string{"-p"}, stdin: "hello", want: "68656c6c6f\n"},
		{name: "nothing at all", stdin: "", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "xxd", test.args, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("xxd: %v", err)
			}
			if got != test.want {
				t.Fatalf("xxd(%q) =\n%q\nwant\n%q", test.stdin, got, test.want)
			}
		})
	}
}

// split's suffixes are what make the pieces sort back into order, so they are
// what the test is about.
func TestSplit(t *testing.T) {
	// Given
	directory := t.TempDir()
	source := filepath.Join(directory, "source")
	if err := os.WriteFile(source, []byte("a\nb\nc\nd\ne\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	applet, ok := applets.DefaultRegistry.Lookup("split")
	if !ok {
		t.Fatal("split is not registered")
	}
	prefix := filepath.Join(directory, "part_")

	// When
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), []string{"-l", "2", source, prefix},
		strings.NewReader(""), &stdout, &stderr)

	// Then
	if err != nil {
		t.Fatalf("split: %v (%s)", err, stderr.String())
	}
	var parts []string
	entries, readErr := os.ReadDir(directory)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "part_") {
			parts = append(parts, entry.Name())
		}
	}
	slices.Sort(parts)
	if !slices.Equal(parts, []string{"part_aa", "part_ab", "part_ac"}) {
		t.Fatalf("parts = %v, want aa, ab and ac", parts)
	}
	first, err := os.ReadFile(filepath.Join(directory, "part_aa"))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != "a\nb\n" {
		t.Fatalf("part_aa = %q, want the first two lines", first)
	}
	// The odd last line gets a piece of its own rather than being dropped.
	last, err := os.ReadFile(filepath.Join(directory, "part_ac"))
	if err != nil {
		t.Fatal(err)
	}
	if string(last) != "e\n" {
		t.Fatalf("part_ac = %q, want the leftover line", last)
	}
}
