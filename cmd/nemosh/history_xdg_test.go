package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withXDGHome is withoutHISTFILE with no XDG variable set but the one given, so the machine's
// own cannot decide where a test's history goes.
func withXDGHome(t *testing.T, variable, value string) string {
	t.Helper()
	home := withoutHISTFILE(t)
	for _, name := range []string{"XDG_STATE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		if old, ok := os.LookupEnv(name); ok {
			os.Unsetenv(name)
			t.Cleanup(func() { os.Setenv(name, old) })
		}
	}
	if variable != "" {
		t.Setenv(variable, value)
	}
	return home
}

// History is state, so it goes in $XDG_STATE_HOME/nemosh, or ~/.local/state/nemosh, as the
// user's tools keep theirs under the home directory on Windows as everywhere. A variable is
// honoured only when it is absolute, as the XDG specification has it. ~/.nemosh_history, where
// every release up to 1.5 kept it, is still read when it is the only one there, and said once;
// `nemosh --migrate` moves it. A fresh home never touches it.
func TestHistoryFile_isStateUnderTheHomeDirectory(t *testing.T) {
	state := filepath.ToSlash(t.TempDir())
	for _, test := range []struct {
		name, variable, value string
		legacy, current       bool
		want                  string // relative to home, or "state" for the variable's
		notice                bool
	}{
		{name: "a fresh home", want: ".local/state/nemosh/history"},
		{name: "XDG_STATE_HOME", variable: "XDG_STATE_HOME", value: state, want: "state"},
		{name: "a relative XDG_STATE_HOME is ignored", variable: "XDG_STATE_HOME", value: "rel/state", want: ".local/state/nemosh/history"},
		{name: "the old file alone is read, and said", legacy: true, want: ".nemosh_history", notice: true},
		{name: "the new file wins over the old", legacy: true, current: true, want: ".local/state/nemosh/history"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := withXDGHome(t, test.variable, test.value)
			if test.legacy {
				writeFile(t, filepath.Join(home, ".nemosh_history"), "old\n")
			}
			if test.current {
				writeFile(t, filepath.Join(home, ".local", "state", "nemosh", "history"), "new\n")
			}

			// When
			got := runInteractiveTest(strings.NewReader("echo \"[$HISTFILE]\"\n"))

			// Then
			want := filepath.ToSlash(home) + "/" + test.want
			if test.want == "state" {
				want = state + "/nemosh/history"
			}
			if hist := strings.TrimSuffix(strings.TrimPrefix(got.stdout, "["), "]\n"); filepath.ToSlash(hist) != want {
				t.Errorf("HISTFILE is %q, want %q", hist, want)
			}
			said := strings.Contains(got.stderr, `run "nemosh --migrate" to move it to `)
			if said != test.notice || strings.Count(got.stderr, "--migrate") > 1 {
				t.Errorf("stderr %q, want the old location said %v, once", got.stderr, test.notice)
			}
			if !test.legacy {
				if _, err := os.Stat(filepath.Join(home, ".nemosh_history")); err == nil {
					t.Errorf("a home without the old file has one now")
				}
			}
		})
	}
}

// The directory is made when the first line is saved, not when the shell starts, so a shell
// that saves nothing leaves nothing behind. Only the default's: a HISTFILE of the user's in a
// directory that is not there saves nothing, as bash's does.
func TestHistoryFile_makesItsDirectoryOnTheFirstWrite(t *testing.T) {
	home := t.TempDir()
	file := newHistoryFile(lookupFrom(map[string]string{"HOME": home}), nil)
	directory := filepath.Join(home, ".local", "state", "nemosh")
	if _, err := os.Stat(directory); err == nil {
		t.Fatal("the directory is there before anything is saved")
	}

	// When
	file.append("echo saved")

	// Then
	data, err := os.ReadFile(filepath.Join(directory, "history"))
	if err != nil || string(data) != "echo saved\n" {
		t.Fatalf("the history file holds %q, %v; want the line saved", data, err)
	}
	mine := filepath.Join(home, "missing", "hist")
	newHistoryFile(lookupFrom(map[string]string{"HOME": home, "HISTFILE": mine}), nil).append("echo lost")
	if _, err := os.Stat(filepath.Dir(mine)); err == nil {
		t.Fatal("a HISTFILE's missing directory was made")
	}
	// Nor a HOME that is not there, which the gate's HOME is not.
	gone := filepath.Join(home, "no-such-home")
	newHistoryFile(lookupFrom(map[string]string{"HOME": gone}), nil).append("echo lost")
	if _, err := os.Stat(gone); err == nil {
		t.Fatal("a HOME that was not there was made by saving a line")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
