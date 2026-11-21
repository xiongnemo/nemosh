package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A session in bash mode reads bash's startup files and then nemosh's, the user's decision of
// 2026-11-21, so one ~/.bashrc serves Git's bash and this shell and ~/.nemoshrc has the last
// word: an interactive one ~/.bashrc, a login one the first of ~/.bash_profile, ~/.bash_login
// and ~/.profile, as bash reads them, and each then $ENV or ~/.nemoshrc. --rcfile names the
// file in ~/.bashrc's place and --norc reads neither. Under the nemosh name, with no
// NEMOSH_COMPAT, only nemosh's are read, as before.
func TestSession_bashModeReadsBashsStartupFilesThenNemoshs(t *testing.T) {
	home := t.TempDir()
	for name, text := range map[string]string{
		".bashrc": "echo bashrc\n", ".nemoshrc": "echo nemoshrc\n", ".bash_profile": "echo bash_profile\n",
		".bash_login": "echo bash_login\n", ".profile": "echo profile\n", "other": "echo other\n",
	} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("PS1", "")
	t.Setenv("PS2", "")
	for _, test := range []struct {
		name, compat string
		args         []string
		want         []string
	}{
		{name: "nemosh -i", args: []string{"-i"}, want: []string{"nemoshrc"}},
		{name: "nemosh bash -i", args: []string{"bash", "-i"}, want: []string{"bashrc", "nemoshrc"}},
		{name: "NEMOSH_COMPAT=bash nemosh -i", compat: "bash", args: []string{"-i"}, want: []string{"bashrc", "nemoshrc"}},
		{name: "nemosh bash -l -i", args: []string{"bash", "-l", "-i"}, want: []string{"bash_profile", "nemoshrc"}},
		{name: "nemosh -l -i", args: []string{"-l", "-i"}, want: []string{"profile", "nemoshrc"}},
		{name: "nemosh bash --rcfile other -i", args: []string{"bash", "--rcfile", filepath.Join(home, "other"), "-i"}, want: []string{"other", "nemoshrc"}},
		{name: "nemosh bash --norc -i", args: []string{"bash", "--norc", "-i"}, want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.compat != "" {
				t.Setenv("NEMOSH_COMPAT", test.compat)
			}
			got := runInvocation(t, "", test.args...)
			// /etc/profile is the machine's and may say anything, so only the files here are
			// looked for, in order.
			var read []string
			for _, line := range strings.Split(got.stdout, "\n") {
				switch line {
				case "bashrc", "nemoshrc", "bash_profile", "bash_login", "profile", "other":
					read = append(read, line)
				}
			}
			if strings.Join(read, " ") != strings.Join(test.want, " ") {
				t.Fatalf("read %q, want %q; stderr %q", read, test.want, got.stderr)
			}
		})
	}
}

// Run as bash, a session keeps its history where bash does, in ~/.bash_history, which Git's bash
// shares; and before the first prompt it reads ~/.nemosh_history after it, so a line typed under
// either name can be recalled under the other.
func TestSession_bashModeKeepsHistoryInBashsFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	got := runInvocation(t, `echo "[${HISTFILE-unset}]"`+"\n", "bash", "--norc", "-i")
	if want := "/.bash_history]\n"; !strings.HasSuffix(got.stdout, want) {
		t.Fatalf("in bash mode HISTFILE is %q, want it to end %q", got.stdout, want)
	}
	plain := runInvocation(t, `echo "[${HISTFILE-unset}]"`+"\n", "--norc", "-i")
	if want := "/.nemosh_history]\n"; !strings.HasSuffix(plain.stdout, want) {
		t.Fatalf("under the nemosh name HISTFILE is %q, want it to end %q", plain.stdout, want)
	}
	for name, text := range map[string]string{".bash_history": "from-bash\n", ".nemosh_history": "from-nemosh\n"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	lookup := func(name string) (string, bool) {
		switch name {
		case "HOME":
			return home, true
		case "HISTFILE":
			return home + "/.bash_history", true
		}
		return "", false
	}
	var lines []string
	for _, file := range historySources(lookup, true, nil) {
		lines = append(lines, file.load()...)
	}
	if strings.Join(lines, " ") != "from-bash from-nemosh" {
		t.Fatalf("bash mode reads %q, want bash's history and then nemosh's", lines)
	}
	if sources := historySources(lookup, false, nil); len(sources) != 1 {
		t.Fatalf("out of bash mode %d history files are read, want the one HISTFILE names", len(sources))
	}
}
