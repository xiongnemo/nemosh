package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// runInDirectory runs an applet with the directory as its operand and returns
// stdout. The operand is a native path, which is what a test has.
func runInDirectory(t *testing.T, name string, directory string, args ...string) (string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup(name)
	if !ok {
		t.Fatalf("%s is not registered", name)
	}
	var stdout, stderr bytes.Buffer
	err := applet.Run(context.Background(), append(args, directory), strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		return stdout.String(), err
	}
	_ = stderr
	return stdout.String(), nil
}

// du's arithmetic, and the one property a script depends on.
//
// The totals are *apparent* sizes rounded up to a 1024-byte block, not what the
// filesystem allocated -- GNU reports allocation and the two differ in both
// directions. Measured on this tree: GNU said 5 for the root and 1 for the
// subdirectory, this says 6 and 2. The divergence is documented rather than
// papered over, because Go cannot read allocation size portably and a `du` that
// silently means something slightly different is worse than one that says so.
func TestDu(t *testing.T) {
	// Given: 3000 bytes in the root, three in a subdirectory
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "big"), bytes.Repeat([]byte("x"), 3000), 0o600); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(directory, "sub")
	if err := os.Mkdir(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "small"), []byte("hi\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("-s reports one total", func(t *testing.T) {
		// When
		got, err := runInDirectory(t, "du", directory, "-s")

		// Then
		if err != nil {
			t.Fatalf("du -s: %v", err)
		}
		// 3 blocks for the 3000-byte file, 1 for the 3-byte one, 1 for each of
		// the two directories.
		if fields := strings.Fields(got); len(fields) == 0 || fields[0] != "6" {
			t.Fatalf("du -s = %q, want 6 blocks", got)
		}
	})

	t.Run("the deepest directory comes first", func(t *testing.T) {
		// GNU prints a directory after the ones inside it, and `du | tail -1`
		// being the total is a common enough idiom that pre-order would be a
		// silent difference.
		// When
		got, err := runInDirectory(t, "du", directory)

		// Then
		if err != nil {
			t.Fatalf("du: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(got), "\n")
		if len(lines) != 2 {
			t.Fatalf("du printed %d lines, want two", len(lines))
		}
		if !strings.HasSuffix(lines[0], "sub") {
			t.Fatalf("first line = %q, want the subdirectory", lines[0])
		}
	})

	t.Run("-h uses the largest unit that fits", func(t *testing.T) {
		// When
		got, err := runInDirectory(t, "du", directory, "-sh")

		// Then
		if err != nil {
			t.Fatalf("du -sh: %v", err)
		}
		if fields := strings.Fields(got); len(fields) == 0 || fields[0] != "6K" {
			t.Fatalf("du -sh = %q, want 6K", got)
		}
	})
}

func TestStat(t *testing.T) {
	// Given
	directory := t.TempDir()
	file := filepath.Join(directory, "five.txt")
	if err := os.WriteFile(file, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("a format string is expanded", func(t *testing.T) {
		// When
		got, err := runInDirectory(t, "stat", file, "-c", "%s|%F")

		// Then
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if strings.TrimSpace(got) != "5|regular file" {
			t.Fatalf("stat = %q, want the size and the type", got)
		}
	})

	t.Run("a directory is named as one", func(t *testing.T) {
		// When
		got, err := runInDirectory(t, "stat", directory, "-c", "%F")

		// Then
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if strings.TrimSpace(got) != "directory" {
			t.Fatalf("stat = %q, want directory", got)
		}
	})

	t.Run("a specifier this does not implement is refused", func(t *testing.T) {
		// Leaving `%i` on the line as literal text is the failure worth avoiding:
		// a script would put it in a filename and never find out why.
		// When
		_, err := runInDirectory(t, "stat", file, "-c", "%i")

		// Then
		if err == nil || !strings.Contains(err.Error(), "unsupported format specifier") {
			t.Fatalf("stat -c %%i = %v, want a refusal naming the specifier", err)
		}
	})

	t.Run("the default output is refused with a reason", func(t *testing.T) {
		// GNU's default block is mostly inode numbers, device ids and permission
		// bits in two notations -- fields Windows either has not got or reports
		// through a different API. A block of zeroes would be indistinguishable
		// from a real answer.
		// When
		_, err := runInDirectory(t, "stat", file)

		// Then
		if err == nil || !strings.Contains(err.Error(), "-c FORMAT") {
			t.Fatalf("stat with no -c = %v, want a refusal naming the form that works", err)
		}
	})
}

// ps is a formatter over the process list internal/proc already keeps for pgrep,
// pkill and the kill builtin. The columns are the intersection of what every ps
// prints and what this can actually see: no TTY, no STAT, no TIME, and not the
// command line -- Windows has no controlling terminal in the POSIX sense, and
// reading another process's command line means walking its PEB, which an ordinary
// session may not do for anything it does not own.
func TestPs(t *testing.T) {
	if runtime.GOOS != "windows" {
		// internal/proc lists processes on Windows only, and refuses elsewhere
		// rather than guessing. ps therefore reports that refusal, which is the
		// correct behaviour and not what this test is about.
		t.Skip("the process list is implemented on Windows only")
	}
	// When
	applet, ok := applets.DefaultRegistry.Lookup("ps")
	if !ok {
		t.Fatal("ps is not registered")
	}
	var stdout, stderr bytes.Buffer
	if err := applet.Run(context.Background(), nil, strings.NewReader(""), &stdout, &stderr); err != nil {
		t.Fatalf("ps: %v (%s)", err, stderr.String())
	}

	// Then
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("ps printed %d lines, want a header and at least one process", len(lines))
	}
	if fields := strings.Fields(lines[0]); len(fields) != 2 || fields[0] != "PID" || fields[1] != "COMMAND" {
		t.Fatalf("header = %q, want PID and COMMAND", lines[0])
	}
	// This process is running, so it has to be in the list -- an empty or
	// fabricated list would pass a weaker assertion.
	if !strings.Contains(stdout.String(), ".exe") && !strings.Contains(stdout.String(), ".test") {
		t.Fatalf("ps listed no recognisable process:\n%s", stdout.String())
	}
}
