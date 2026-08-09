package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tr is the one this bundle most needed, and on Windows it is needed twice
// over: `tr -d '\r'` is how a file that crossed platforms gets fixed, and
// nothing else here does it.
func TestTr(t *testing.T) {
	for _, test := range []struct {
		name  string
		args  []string
		input string
		want  string
	}{
		{name: "translate", args: []string{"a-z", "A-Z"}, input: "hello\n", want: "HELLO\n"},
		{name: "strip carriage returns", args: []string{"-d", `\r`}, input: "a\r\nb\r\n", want: "a\nb\n"},
		{name: "delete a set", args: []string{"-d", "aeiou"}, input: "banana\n", want: "bnn\n"},
		{name: "squeeze runs", args: []string{"-s", " "}, input: "a   b\n", want: "a b\n"},
		{name: "a short second set repeats its last", args: []string{"a-z", "A"}, input: "hey\n", want: "AAA\n"},
		{name: "complement and delete", args: []string{"-cd", "0-9\n"}, input: "a1b2c3\n", want: "123\n"},
		{name: "an explicit range", args: []string{"0-4", "5-9"}, input: "0123\n", want: "5678\n"},
		{name: "a literal dash at the end", args: []string{"-d", "a-"}, input: "a-b\n", want: "b\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, _, err := runAppletWithInput(t, test.input, "tr", test.args...)

			// Then
			if err != nil {
				t.Fatalf("tr %v = %v", test.args, err)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}
}

// A character class is refused rather than read as the characters its brackets
// are made of, which is what taking it literally would silently do.
func TestTr_refusesACharacterClass(t *testing.T) {
	_, _, err := runAppletWithInput(t, "abc\n", "tr", "[:alpha:]", "x")
	if err == nil || !strings.Contains(err.Error(), "classes") {
		t.Fatalf("err = %v, want it to refuse the class by name", err)
	}
}

func TestTee(t *testing.T) {
	// Given
	directory := t.TempDir()
	first := filepath.Join(directory, "one.txt")
	second := filepath.Join(directory, "two.txt")

	// When
	stdout, _, err := runAppletWithInput(t, "body\n", "tee", first, second)

	// Then: through to stdout, and into both files.
	if err != nil {
		t.Fatalf("tee = %v", err)
	}
	if stdout != "body\n" {
		t.Fatalf("stdout = %q, want the input passed through", stdout)
	}
	for _, path := range []string{first, second} {
		assertFile(t, path, "body\n")
	}

	// And -a appends rather than truncating.
	if _, _, err := runAppletWithInput(t, "more\n", "tee", "-a", first); err != nil {
		t.Fatalf("tee -a = %v", err)
	}
	assertFile(t, first, "body\nmore\n")
}

func TestSeq(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "to a last", args: []string{"3"}, want: "1\n2\n3\n"},
		{name: "from and to", args: []string{"2", "4"}, want: "2\n3\n4\n"},
		{name: "with an increment", args: []string{"1", "2", "5"}, want: "1\n3\n5\n"},
		{name: "counting down", args: []string{"3", "-1", "1"}, want: "3\n2\n1\n"},
		{name: "an empty range says nothing", args: []string{"3", "1"}, want: ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, _, err := runAppletWithInput(t, "", "seq", test.args...)

			// Then
			if err != nil {
				t.Fatalf("seq %v = %v", test.args, err)
			}
			if stdout != test.want {
				t.Fatalf("stdout = %q, want %q", stdout, test.want)
			}
		})
	}

	// A zero increment would never finish, so it is refused rather than run.
	if _, _, err := runAppletWithInput(t, "", "seq", "1", "0", "5"); err == nil {
		t.Fatal("seq with a zero increment was accepted")
	}
}

func TestClear(t *testing.T) {
	// When
	stdout, _, err := runAppletWithInput(t, "", "clear")

	// Then: home, then erase the display -- the same pair Ctrl-L emits.
	if err != nil {
		t.Fatalf("clear = %v", err)
	}
	if stdout != "\033[H\033[2J" {
		t.Fatalf("stdout = %q, want the clear sequence", stdout)
	}
}

// whoami answers with the shell's own identity, which under elevation is root.
// Windows' whoami.exe says DOMAIN\user and has no notion of root at all.
func TestWhoami(t *testing.T) {
	// When
	stdout, _, err := runAppletWithInput(t, "", "whoami")

	// Then
	if err != nil {
		t.Fatalf("whoami = %v", err)
	}
	name := strings.TrimSpace(stdout)
	if name == "" {
		t.Fatal("whoami said nothing")
	}
	if strings.Contains(name, `\`) {
		t.Fatalf("whoami = %q, want the bare account name rather than DOMAIN\\user", name)
	}
	// And it must agree with id, which is the whole reason the identity is
	// derived in one place.
	fromID, _, err := runAppletWithInput(t, "", "id", "-un")
	if err != nil {
		t.Fatalf("id -un = %v", err)
	}
	if strings.TrimSpace(fromID) != name {
		t.Fatalf("whoami says %q and id -un says %q", name, strings.TrimSpace(fromID))
	}
}

func TestMktemp(t *testing.T) {
	t.Run("makes a file that exists", func(t *testing.T) {
		// Given / When
		directory := t.TempDir()
		stdout, _, err := runAppletWithInput(t, "", "mktemp", filepath.Join(directory, "scratch.XXXXXX"))
		if err != nil {
			t.Fatalf("mktemp = %v", err)
		}

		// Then: creating it is what makes the name safe to use -- a name that is
		// merely unused when it is chosen is a race.
		path := strings.TrimSpace(stdout)
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("mktemp named %q, which does not exist: %v", path, statErr)
		}
		if info.IsDir() {
			t.Fatalf("mktemp made a directory without -d")
		}
		if !strings.Contains(filepath.Base(path), "scratch.") {
			t.Fatalf("mktemp ignored the template: %q", path)
		}
	})

	t.Run("-d makes a directory", func(t *testing.T) {
		// Given / When
		directory := t.TempDir()
		stdout, _, err := runAppletWithInput(t, "", "mktemp", "-d", filepath.Join(directory, "scratch.XXXXXX"))
		if err != nil {
			t.Fatalf("mktemp -d = %v", err)
		}

		// Then
		info, statErr := os.Stat(strings.TrimSpace(stdout))
		if statErr != nil || !info.IsDir() {
			t.Fatalf("mktemp -d named %q, which is not a directory (%v)", stdout, statErr)
		}
	})

	t.Run("twice gives two names", func(t *testing.T) {
		// Given / When
		directory := t.TempDir()
		first, _, err := runAppletWithInput(t, "", "mktemp", filepath.Join(directory, "x.XXXXXX"))
		if err != nil {
			t.Fatal(err)
		}
		second, _, err := runAppletWithInput(t, "", "mktemp", filepath.Join(directory, "x.XXXXXX"))
		if err != nil {
			t.Fatal(err)
		}

		// Then
		if first == second {
			t.Fatalf("mktemp gave the same name twice: %q", first)
		}
	})
}
