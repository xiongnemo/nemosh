package runtime_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `mapfile` and `readarray`, measured against bash on the same input.
//
// The one worth stating is that **the delimiter is kept by default and `-t` strips it**,
// which is the opposite of what most people expect. Reading a file without `-t` gives
// elements that still end in their newline, and a script that forgets prints blank lines
// between everything.

func mapfileFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(path)
}

func TestMapfile_readsAStreamIntoAnArray(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
		script  string
		want    string
	}{
		{
			name: "one element per line", content: "one\ntwo\nthree\n",
			script: `mapfile -t x < @IN@; printf '[%s]' "${x[@]}"; echo`,
			want:   "[one][two][three]\n",
		},
		{
			// Without -t the delimiter is part of the element, which is bash's rule
			// and the thing people get wrong.
			name: "the newline is kept without -t", content: "one\ntwo\n",
			script: `mapfile x < @IN@; printf '[%s]' "${x[@]}"; echo`,
			want:   "[one\n][two\n]\n",
		},
		{
			// A last line with no terminator is still a line. It arrives from the
			// reader together with the end-of-stream error, so the naive loop that
			// checks the error first drops it.
			name: "a final line with no newline", content: "no-eol-here",
			script: `mapfile -t x < @IN@; printf '[%s]' "${x[@]}"; echo`,
			want:   "[no-eol-here]\n",
		},
		{
			name: "readarray is the same builtin", content: "one\ntwo\nthree\n",
			script: `readarray -t x < @IN@; echo ${#x[@]}`,
			want:   "3\n",
		},
		{
			name: "an empty stream gives an empty array", content: "",
			script: `mapfile -t x < @IN@; echo "n=${#x[@]}"`,
			want:   "n=0\n",
		},
		{
			// The default name, which makes `mapfile < f` on its own a real idiom.
			name: "no name means MAPFILE", content: "one\ntwo\n",
			script: `mapfile -t < @IN@; echo "${MAPFILE[@]}"`,
			want:   "one two\n",
		},

		// The counting options.
		{
			name: "-n stops early", content: "one\ntwo\nthree\n",
			script: `mapfile -t -n 2 x < @IN@; echo "${x[@]}"`,
			want:   "one two\n",
		},
		{
			name: "-s skips", content: "one\ntwo\nthree\n",
			script: `mapfile -t -s 1 x < @IN@; echo "${x[@]}"`,
			want:   "two three\n",
		},
		{
			// -O assigns *into* the array rather than replacing it, so the indices
			// start where it was told to and earlier ones are untouched.
			name: "-O starts at an index", content: "one\ntwo\nthree\n",
			script: `mapfile -t -O 2 x < @IN@; echo "${!x[@]}"`,
			want:   "2 3 4\n",
		},
		{
			name: "-n0 means everything", content: "one\ntwo\nthree\n",
			script: `mapfile -t -n 0 x < @IN@; echo ${#x[@]}`,
			want:   "3\n",
		},
		{
			// Joined and separate forms of a valued option both work, as in bash.
			name: "-n3 joined", content: "one\ntwo\nthree\nfour\n",
			script: `mapfile -t -n3 x < @IN@; echo ${#x[@]}`,
			want:   "3\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := mapfileFixture(t, "input.txt", test.content)
			script := strings.ReplaceAll(test.script, "@IN@", path)
			status, stdout, stderr := runSetScript(t, script+"\n")
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr)
			}
			if stdout != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.script, stdout, test.want)
			}
		})
	}
}

// A delimiter other than a newline, including the empty one that means NUL -- which is
// how `find -print0` output is read, and the reason -d exists at all.
func TestMapfile_delimiter(t *testing.T) {
	status, stdout, stderr := runSetScript(t,
		"mapfile -t -d : x <<< 'a:b:c'\nprintf '[%s]' \"${x[@]}\"\necho\n")
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if want := "[a][b][c\n]\n"; stdout != want {
		t.Fatalf("got %q, want %q", stdout, want)
	}

	// An empty -d is the NUL byte, which is how `find -print0` output is read and the
	// reason the option exists. The fixture holds real NUL bytes, written from Go --
	// a shell redirect could not carry them and AGENTS.md already records a fixture
	// that was silently wrong for being built by the shell.
	// Named `zeroes.bin` and not `nul.bin`, which is what this was first called: **NUL
	// is a reserved device name on Windows and stays reserved with an extension**, so
	// `nul.bin` is the null device. The fixture wrote four bytes into nowhere and read
	// back empty, and the failure looked exactly like mapfile ignoring its delimiter.
	// This project refuses that name inside archives for the same reason
	// (safeArchivePath); it is just as true of a test fixture.
	zeroes := mapfileFixture(t, "zeroes.bin", "a\x00b\x00")
	if onDisk, err := os.ReadFile(zeroes); err != nil || len(onDisk) != 4 {
		t.Fatalf("the fixture is %d bytes (%v), want 4 -- the test, not the shell", len(onDisk), err)
	}
	status, stdout, stderr = runSetScript(t,
		"mapfile -t -d '' x < "+zeroes+"\necho ${#x[@]} ${x[0]}\n")
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr)
	}
	if want := "2 a\n"; stdout != want {
		t.Fatalf("a NUL delimiter gave %q, want %q (stderr %q)", stdout, want, stderr)
	}
}

// The refusals. `-C` is the one that matters: bash runs a callback every N lines, and a
// script that passes it expects its function to run -- silently not running it is the
// quiet wrongness this project refuses everywhere else.
func TestMapfile_refusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		says   string
	}{
		{name: "a callback", script: `mapfile -C handler -c 2 x < /dev/null`, says: "callbacks are not implemented"},
		{name: "an option that is not one", script: `mapfile -Z x < /dev/null`, says: "invalid option"},
		{name: "two names", script: `mapfile x y < /dev/null`, says: "too many arguments"},
		{name: "a count that is not a number", script: `mapfile -n abc x < /dev/null`, says: "invalid number"},
		{name: "a missing option argument", script: `mapfile -n`, says: "requires an argument"},
		{name: "a readonly name", script: "readonly x\nmapfile x < /dev/null", says: "readonly"},
	} {
		t.Run(test.name, func(t *testing.T) {
			status, _, stderr := runSetScript(t, test.script+"\n")
			if status == 0 {
				t.Fatalf("%q succeeded", test.script)
			}
			if !strings.Contains(stderr, test.says) {
				t.Fatalf("%q said %q, which does not contain %q", test.script, stderr, test.says)
			}
		})
	}
}
