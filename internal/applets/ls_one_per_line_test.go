package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// -1 is one of the most-typed ls options there is, and busybox has it. This ls
// always writes one entry per line, so -1 asks for the format already in use --
// accepting it is reporting what happens, not pretending.
//
// The distinction that matters is -C, which asks for the thing this cannot do
// and is still refused. An option that changes nothing may be accepted; one that
// would change everything may not be quietly ignored.
func TestLs_acceptsOnePerLine_andStillRefusesColumns(t *testing.T) {
	// Given
	directory := t.TempDir()
	for _, name := range []string{"alpha.txt", "beta.txt"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// When
	plain, _, err := runAppletWithInput(t, "", "ls", directory)
	if err != nil {
		t.Fatalf("ls = %v", err)
	}
	onePerLine, _, err := runAppletWithInput(t, "", "ls", "-1", directory)
	if err != nil {
		t.Fatalf("ls -1 = %v, want it accepted", err)
	}

	// Then
	if onePerLine != plain {
		t.Fatalf("ls -1 gave %q and ls gave %q; -1 asks for the format already in use", onePerLine, plain)
	}
	if want := "alpha.txt\nbeta.txt\n"; plain != want {
		t.Fatalf("ls gave %q, want %q", plain, want)
	}

	// -C is implemented now, and it is the only way to see the layout from a test at all:
	// the destination decides the format, and a test's destination is never a terminal.
	// This assertion used to require that -C be *refused*.
	columns, _, err := runAppletWithInput(t, "", "ls", "-C", directory)
	if err != nil {
		t.Fatalf("ls -C = %v, want it accepted", err)
	}
	if want := "alpha.txt  beta.txt\n"; columns != want {
		t.Fatalf("ls -C gave %q, want %q", columns, want)
	}
}

// busybox resolves the pair the same way whichever order they arrive in: -l
// wins, and -1 never disturbs it.
func TestLs_longFormatWinsOverOnePerLine_inEitherOrder(t *testing.T) {
	// Given
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "alpha.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// When
	long, _, err := runAppletWithInput(t, "", "ls", "-l", directory)
	if err != nil {
		t.Fatalf("ls -l = %v", err)
	}

	// Then
	for _, args := range [][]string{{"-l", "-1"}, {"-1", "-l"}, {"-l1"}, {"-1l"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			got, _, err := runAppletWithInput(t, "", "ls", append(args, directory)...)
			if err != nil {
				t.Fatalf("ls %v = %v", args, err)
			}
			if got != long {
				t.Fatalf("ls %v gave %q, want the long format %q", args, got, long)
			}
		})
	}
}
