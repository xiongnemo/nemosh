package main

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A user's completion specs are configuration, so they are kept in $XDG_CONFIG_HOME/nemosh/
// completions, ~/.config/nemosh/completions by default, on Windows too. %APPDATA%\nemosh\
// completions, which releases up to 1.5 looked in first, is looked in after it, so each spec is
// found where it is and the new place wins for one in both; a variable that is not absolute is
// ignored, as the XDG specification has it, and USERPROFILE is the home when HOME is not set.
func TestSpecSearchDirectories_newPlaceFirstThenTheOld(t *testing.T) {
	home, appData, config := t.TempDir(), t.TempDir(), t.TempDir()
	newPlace := filepath.Join(home, ".config", "nemosh", "completions")
	oldPlace := filepath.Join(appData, "nemosh", "completions")
	for _, test := range []struct {
		name string
		env  map[string]string
		want []string
	}{
		{name: "both", env: map[string]string{"HOME": home, "APPDATA": appData}, want: []string{newPlace, oldPlace}},
		{name: "no APPDATA", env: map[string]string{"HOME": home}, want: []string{newPlace}},
		{name: "XDG_CONFIG_HOME", env: map[string]string{"HOME": home, "XDG_CONFIG_HOME": config},
			want: []string{filepath.Join(config, "nemosh", "completions")}},
		{name: "a relative XDG_CONFIG_HOME is ignored", env: map[string]string{"HOME": home, "XDG_CONFIG_HOME": "rel"}, want: []string{newPlace}},
		{name: "USERPROFILE without HOME", env: map[string]string{"USERPROFILE": home}, want: []string{newPlace}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := specSearchDirectories(lookupFrom(test.env)); !slices.Equal(got, test.want) {
				t.Fatalf("searched %q, want %q", got, test.want)
			}
		})
	}
}

// A spec only at the old place is still used, and a session says so once before its first
// prompt; nothing is said for an old place with no spec in it.
func TestSayOldSpecs_saysWhenASpecIsOnlyAtTheOldPlace(t *testing.T) {
	home, appData := t.TempDir(), t.TempDir()
	env := lookupFrom(map[string]string{"HOME": home, "APPDATA": appData})
	var quiet, said bytes.Buffer
	sayOldSpecs(&quiet, env)
	writeFile(t, filepath.Join(appData, "nemosh", "completions", "tool.toml"), "")
	sayOldSpecs(&said, env)

	if quiet.Len() != 0 {
		t.Errorf("with no old spec it said %q", quiet.String())
	}
	if want := `run "nemosh --migrate" to move it to `; !strings.Contains(said.String(), want) || strings.Count(said.String(), "\n") != 1 {
		t.Errorf("with an old spec it said %q", said.String())
	}
}

// --migrate merges the old directory into the new one a file at a time, never over one, and
// removes what it leaves empty and owns -- %APPDATA%\nemosh, never %APPDATA%.
func TestMigrate_mergesTheOldSpecDirectory(t *testing.T) {
	home, appData := t.TempDir(), t.TempDir()
	env := map[string]string{"HOME": home, "APPDATA": appData}
	oldPlace := filepath.Join(appData, "nemosh", "completions")
	newPlace := filepath.Join(home, ".config", "nemosh", "completions")
	writeFile(t, filepath.Join(oldPlace, "only-old.toml"), "old\n")
	writeFile(t, filepath.Join(oldPlace, "same.toml"), "same\n")
	writeFile(t, filepath.Join(newPlace, "same.toml"), "same\n")
	writeFile(t, filepath.Join(newPlace, "only-new.toml"), "new\n")

	stdout, stderr, err := migrateIn(t, env)

	if err != nil {
		t.Fatalf("%v; %q %q", err, stdout, stderr)
	}
	for name, want := range map[string]string{"only-old.toml": "old\n", "same.toml": "same\n", "only-new.toml": "new\n"} {
		if got := readOrEmpty(filepath.Join(newPlace, name)); got != want {
			t.Errorf("%s holds %q, want %q", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(appData, "nemosh")); err == nil {
		t.Errorf("%%APPDATA%%\\nemosh is still there, empty")
	}
	if _, err := os.Stat(appData); err != nil {
		t.Errorf("%%APPDATA%% itself went: %v", err)
	}
	if stdout, _, err := migrateIn(t, env); err != nil || !strings.Contains(stdout, "nothing to migrate") {
		t.Errorf("a second run: %q, %v", stdout, err)
	}
}
