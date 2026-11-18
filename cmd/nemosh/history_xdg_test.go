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

// History is kept in ~/.nemosh_history, as busybox's ash keeps ~/.ash_history and bash
// ~/.bash_history: a shell's own files stay where a shell's are, history as the rc file, and
// XDG_STATE_HOME does not move it, as it does not move bash's. The nightlies 1.5.2 and 1.5.3 kept
// it in $XDG_STATE_HOME/nemosh/nemosh_history, or ~/.local/state/nemosh/nemosh_history; a file
// there is still read when it is the only one, and said once, and `nemosh --migrate` moves it
// back. A home without one is given no ~/.local.
func TestHistoryFile_isInTheHomeDirectory(t *testing.T) {
	state := filepath.ToSlash(t.TempDir())
	for _, test := range []struct {
		name, variable, value string
		home, nightly         bool
		want                  string // relative to home, or "state" for the variable's
		notice                bool
	}{
		{name: "a fresh home", want: ".nemosh_history"},
		{name: "XDG_STATE_HOME does not move it", variable: "XDG_STATE_HOME", value: state, want: ".nemosh_history"},
		{name: "a nightly's file alone is read, and said", nightly: true, want: ".local/state/nemosh/nemosh_history", notice: true},
		{name: "a nightly's file under XDG_STATE_HOME alone is read, and said", variable: "XDG_STATE_HOME", value: state, nightly: true, want: "state", notice: true},
		{name: "a relative XDG_STATE_HOME is ignored", variable: "XDG_STATE_HOME", value: "rel/state", nightly: true, want: ".local/state/nemosh/nemosh_history", notice: true},
		{name: "the home's file wins over a nightly's", home: true, nightly: true, want: ".nemosh_history"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := withXDGHome(t, test.variable, test.value)
			if test.home {
				writeFile(t, filepath.Join(home, ".nemosh_history"), "home\n")
			}
			if test.nightly {
				if test.want == "state" {
					writeFile(t, filepath.Join(state, "nemosh", "nemosh_history"), "nightly\n")
				} else {
					writeFile(t, filepath.Join(home, ".local", "state", "nemosh", "nemosh_history"), "nightly\n")
				}
			}

			// When
			got := runInteractiveTest(strings.NewReader("echo \"[$HISTFILE]\"\n"))

			// Then
			want := filepath.ToSlash(home) + "/" + test.want
			if test.want == "state" {
				want = state + "/nemosh/nemosh_history"
			}
			if hist := strings.TrimSuffix(strings.TrimPrefix(got.stdout, "["), "]\n"); filepath.ToSlash(hist) != want {
				t.Errorf("HISTFILE is %q, want %q", hist, want)
			}
			said := strings.Contains(got.stderr, `run "nemosh --migrate" to move it to `)
			if said != test.notice || strings.Count(got.stderr, "--migrate") > 1 {
				t.Errorf("stderr %q, want the old location said %v, once", got.stderr, test.notice)
			}
			notice, _, _ := strings.Cut(got.stderr, "\n")
			if test.notice && !strings.HasSuffix(filepath.ToSlash(notice), " to move it to "+filepath.ToSlash(home)+"/.nemosh_history") {
				t.Errorf("the notice is %q, want it to say history goes to ~/.nemosh_history", notice)
			}
			if !test.nightly {
				if _, err := os.Stat(filepath.Join(home, ".local")); err == nil {
					t.Errorf("a home without a nightly's file has a ~/.local now")
				}
			}
		})
	}
}

// Saving a line makes no directory. The default is in HOME, which is there or saves nothing, as
// bash's does; a HISTFILE of the user's in a directory that is not there saves nothing too.
func TestHistoryFile_makesNoDirectoryToSaveALine(t *testing.T) {
	home := t.TempDir()

	// When
	newHistoryFile(lookupFrom(map[string]string{"HOME": home}), nil).append("echo saved")

	// Then
	data, err := os.ReadFile(filepath.Join(home, ".nemosh_history"))
	if err != nil || string(data) != "echo saved\n" {
		t.Fatalf("~/.nemosh_history holds %q, %v; want the line saved", data, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".local")); err == nil {
		t.Fatal("saving a line made ~/.local")
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
