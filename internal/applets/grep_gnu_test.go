package applets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// GNU grep's --include, --exclude, --exclude-dir, -a, -I, the binary-file answer and --color,
// measured against GNU grep. busybox's grep has none of them; see grep_long.go.

// grepTree is a.go, b.txt, sub/c.go, skip/d.go and bin.dat, each with a needle in it.
func grepTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range map[string]string{
		"a.go": "needle in go\n", "b.txt": "needle in txt\n", "sub/c.go": "needle deep\n",
		"skip/d.go": "needle skipped\n", "bin.dat": "needle\x00\x01\n",
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// sortedLines is output a line at a time, in order, with the tree's directory taken off the
// front of each, since a walk's order is the filesystem's.
func sortedLines(output, dir string) []string {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(output, filepath.ToSlash(dir), "")), "\n")
	for left := range lines {
		for right := left + 1; right < len(lines); right++ {
			if lines[right] < lines[left] {
				lines[left], lines[right] = lines[right], lines[left]
			}
		}
	}
	return lines
}

func TestGrep_includeExcludeAndExcludeDirChooseWhatTheWalkReads(t *testing.T) {
	dir := grepTree(t)
	root := filepath.ToSlash(dir)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "--include=GLOB", args: []string{"--include=*.go"}, want: "/a.go:needle in go|/skip/d.go:needle skipped|/sub/c.go:needle deep"},
		{name: "--include GLOB, the value a word of its own", args: []string{"--include", "*.go"}, want: "/a.go:needle in go|/skip/d.go:needle skipped|/sub/c.go:needle deep"},
		{name: "--exclude", args: []string{"--exclude=*.go", "--exclude=*.dat"}, want: "/b.txt:needle in txt"},
		{name: "--exclude-dir", args: []string{"--exclude-dir=skip", "--include=*.go"}, want: "/a.go:needle in go|/sub/c.go:needle deep"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, stderr, status := runApplet(t, "grep", append(append([]string{"-r"}, test.args...), "needle", root), "")

			// Then
			if got := strings.Join(sortedLines(stdout, root), "|"); got != test.want || status != 0 {
				t.Fatalf("got %q, status %d, stderr %q; want %q", got, status, stderr, test.want)
			}
		})
	}
}

// A binary file that matches is said to, on stderr, as GNU 3.5 and later say it, and its bytes
// are not printed; status 0. -a prints the line, -I reads it as holding no match, and -c counts.
func TestGrep_aBinaryFileIsSaidToMatchRatherThanPrinted(t *testing.T) {
	path := filepath.Join(grepTree(t), "bin.dat")
	for _, test := range []struct {
		name           string
		args           []string
		stdout, stderr string
		status         int
	}{
		{name: "detected", args: []string{"needle", path}, stderr: "grep: " + path + ": binary file matches\n"},
		{name: "-a", args: []string{"-a", "needle", path}, stdout: "needle\x00\x01\n"},
		{name: "--text", args: []string{"--text", "needle", path}, stdout: "needle\x00\x01\n"},
		{name: "-I", args: []string{"-I", "needle", path}, status: 1},
		{name: "-c", args: []string{"-c", "needle", path}, stdout: "1\n"},
		{name: "-l", args: []string{"-l", "needle", path}, stdout: path + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, stderr, status := runApplet(t, "grep", test.args, "")

			// Then
			if stdout != test.stdout || stderr != test.stderr || status != test.status {
				t.Fatalf("stdout %q, stderr %q, status %d; want %q, %q, %d", stdout, stderr, status, test.stdout, test.stderr, test.status)
			}
		})
	}
	// From standard input it is named as GNU names it.
	if _, stderr, _ := runApplet(t, "grep", []string{"needle"}, "needle\x00\n"); stderr != "grep: (standard input): binary file matches\n" {
		t.Fatalf("from stdin: %q", stderr)
	}
	// And UTF-16 text, which is half NULs as it arrives, is text.
	utf16 := "\xff\xfen\x00e\x00e\x00d\x00l\x00e\x00\n\x00"
	if stdout, _, _ := runApplet(t, "grep", []string{"needle"}, utf16); stdout != "needle\n" {
		t.Fatalf("UTF-16 text came out as %q", stdout)
	}
}

// --color=always is GNU's colours, byte for byte as GNU grep 3.0 writes them; --color=auto into
// something that is not a terminal is none, as --color=never is.
func TestGrep_colourIsGNUsWhereAsked(t *testing.T) {
	dir := grepTree(t)
	a, b := filepath.Join(dir, "a.go"), filepath.Join(dir, "b.txt")
	name := func(path string) string { return "\x1b[35m\x1b[K" + path + "\x1b[m\x1b[K" }
	sep := "\x1b[36m\x1b[K:\x1b[m\x1b[K"
	line := "\x1b[32m\x1b[K1\x1b[m\x1b[K"
	match := "\x1b[01;31m\x1b[Kneedle\x1b[m\x1b[K"
	want := name(a) + sep + line + sep + match + " in go\n" + name(b) + sep + line + sep + match + " in txt\n"

	// When
	stdout, _, _ := runApplet(t, "grep", []string{"--color=always", "-n", "needle", a, b}, "")

	// Then
	if stdout != want {
		t.Fatalf("got\n%q\nwant\n%q", stdout, want)
	}
	for _, when := range []string{"--color=auto", "--color=never", "--colour=auto"} {
		if plain, _, _ := runApplet(t, "grep", []string{when, "needle", a}, ""); plain != "needle in go\n" {
			t.Fatalf("%s into a buffer gave %q", when, plain)
		}
	}
	// Under -w the word is coloured, not the blanks the pattern holds around it, and two words a
	// blank apart are both coloured.
	words, _, _ := runApplet(t, "grep", []string{"--color=always", "-w", "ab"}, "ab ab x\n")
	if strings.Count(words, "\x1b[01;31m\x1b[Kab\x1b[m\x1b[K") != 2 || !strings.HasSuffix(words, " x\n") {
		t.Fatalf("-w coloured %q", words)
	}
}
