package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// **Who is allowed to change the terminal, and for how long.**
//
// Written after the prompt was found holding the terminal raw across every command it ran,
// which made `bc` at the prompt look frozen: no echo, no line ever completed, and Ctrl-C
// ignored. The fix was to scope the borrow; this is what stops the scope being widened again
// somewhere else.
//
// Every place in the tree that changes console state is listed here with the reason it is
// allowed, and each reason is a different shape:
//
//   - **interactive_lineedit.go** borrows raw mode, and gives it back after one line read.
//     The editor needs to see arrows; a command needs the terminal it expects. See
//     raw_mode_scope_test.go, which holds that scope specifically.
//   - **hold_option.go** borrows it for `--hold`, which waits for one key at exit, and gives
//     it back in the same function. Nothing runs in between.
//   - **virtual_terminal_windows.go** turns on ENABLE_VIRTUAL_TERMINAL_PROCESSING for the
//     whole session, and that one *should* be session-long: it is an output mode, and the
//     commands want it too -- it is what makes colour work at all.
//   - **stty_windows.go** and **stty_other.go** change it because somebody typed `stty`.
//     That is the applet's whole purpose.
//
// Anything else that needs to read keys goes through tcell, which saves the mode it finds
// and restores it on the way out: the editors, `top`, and `less`. An applet reading keys by
// hand would be the way this bug comes back, and there is no reason to.

func TestTerminalState_isChangedOnlyWhereItIsOwned(t *testing.T) {
	allowed := map[string]string{
		"cmd/nemosh/interactive_lineedit.go":     "borrows raw mode for one line read",
		"cmd/nemosh/hold_option.go":              "one key at exit, returned in the same function",
		"cmd/nemosh/virtual_terminal_windows.go": "an output mode the whole session wants",
		"internal/applets/stty_windows.go":       "stty was asked for",
		"internal/applets/stty_other.go":         "stty was asked for",
	}
	// The calls that mean "take the terminal over".
	markers := []string{"term.MakeRaw", "term.Restore", "SetConsoleMode", "IoctlSetTermios"}

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// references/ carries vendored C for citation, and it is not this tree's code.
			if name := entry.Name(); name == "references" || name == ".git" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		checked++
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		for _, marker := range markers {
			if !strings.Contains(string(data), marker) {
				continue
			}
			if _, ok := allowed[relative]; !ok {
				t.Errorf("%s calls %s; terminal state is borrowed in %d named places and "+
					"given back at once, because a command that runs while it is held gets a "+
					"terminal it cannot read. Use tcell if the applet needs keys.",
					relative, marker, len(allowed))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// A walk that found nothing would pass in silence, which is the failure this project
	// has paid for before.
	if checked < 200 {
		t.Fatalf("only %d files were scanned, so this proves nothing", checked)
	}
}
