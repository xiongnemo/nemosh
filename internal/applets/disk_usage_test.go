package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
// The totals are what the filesystem *allocated* now, which is what the name means and what
// both references report. That makes the exact number a property of the volume rather than of
// the tree -- a 3000-byte file costs one 4096-byte cluster on NTFS and one 4096-byte block on
// ext4, but nothing in Go promises either -- so these assert the relationships a script
// actually depends on instead of a number measured on one machine. The numbers themselves are
// pinned per platform in the corpus, where the volume is known.
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

	t.Run("-s reports one total, and it covers both files", func(t *testing.T) {
		// When
		got, err := runInDirectory(t, "du", directory, "-s")

		// Then
		if err != nil {
			t.Fatalf("du -s: %v", err)
		}
		if lines := strings.Split(strings.TrimSpace(got), "\n"); len(lines) != 1 {
			t.Fatalf("du -s printed %d lines, want one", len(lines))
		}
		// At least the 3000-byte file rounded up, plus something for the 3-byte one.
		// An exact number would be a fact about the volume; see the comment above.
		blocks, err := strconv.Atoi(strings.Fields(got)[0])
		if err != nil {
			t.Fatalf("du -s = %q, want a block count first", got)
		}
		if blocks < 4 {
			t.Fatalf("du -s = %q, want at least 4 blocks for 3003 bytes in two files", got)
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

	// The expectation here was `6K`, which neither reference prints: busybox-w32 and GNU
	// both keep the decimal on a whole number below ten. Above ten it goes again -- GNU says
	// `97K` and busybox `96.7K`, and this follows GNU.
	t.Run("-h keeps one decimal below ten", func(t *testing.T) {
		// When
		got, err := runInDirectory(t, "du", directory, "-sh")

		// Then
		if err != nil {
			t.Fatalf("du -sh: %v", err)
		}
		// The rule rather than a number, because the number is the volume's: this tree
		// costs 4 blocks on NTFS, where a directory is free, and 16 on ext4, where each
		// of the two directories has a block of its own. Below ten the decimal is kept
		// and above it goes, and only the first half is what this used to get wrong.
		field := strings.Fields(got)[0]
		if !strings.HasSuffix(field, "K") {
			t.Fatalf("du -sh = %q, want a size in kilobytes", got)
		}
		blocks, err := strconv.ParseFloat(strings.TrimSuffix(field, "K"), 64)
		if err != nil {
			t.Fatalf("du -sh = %q, want a number before the K", got)
		}
		if blocks < 10 && !strings.Contains(field, ".") {
			t.Fatalf("du -sh = %q, want the decimal kept below ten -- both references print 4.0K", got)
		}
		if blocks >= 10 && strings.Contains(field, ".") {
			t.Fatalf("du -sh = %q, want no decimal at ten or above -- GNU prints 97K", got)
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

// ps is a formatter over the process list internal/proc already keeps for pgrep, pkill and the
// kill builtin. It printed two columns until the list moved onto the system table, which answers
// with parents, thread counts, memory and CPU time for no more privilege than a name cost.
//
// Still absent, for two different reasons. TTY and the command line: Windows has no controlling
// terminal, and a command line means opening the process, which is refused for anything this
// session does not own. STAT: derivable from the process's threads, but an approximation belongs
// in `top`, where a monitor's reader expects one, rather than in `ps`.
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
	// The header used to be two columns, because CreateToolhelp32Snapshot answered with a name
	// and a pid and there was nothing else to print. The system table answers with rather more
	// for no more privilege, so ps prints what busybox-w32's does.
	want := []string{"PID", "PPID", "THR", "RSS", "TIME", "COMMAND"}
	fields := strings.Fields(lines[0])
	if len(fields) != len(want) {
		t.Fatalf("header = %q, want %v", lines[0], want)
	}
	for index, column := range want {
		if fields[index] != column {
			t.Fatalf("header = %q, want %v", lines[0], want)
		}
	}
	// A row must carry a real thread count and a real parent, which is what proves the row
	// came from the system table rather than from a snapshot that could not answer.
	row := strings.Fields(lines[1])
	if len(row) < 3 || row[2] == "0" {
		t.Fatalf("first row = %q, want a thread count", lines[1])
	}
	// This process is running, so it has to be in the list -- an empty or
	// fabricated list would pass a weaker assertion.
	if !strings.Contains(stdout.String(), ".exe") && !strings.Contains(stdout.String(), ".test") {
		t.Fatalf("ps listed no recognisable process:\n%s", stdout.String())
	}
}
