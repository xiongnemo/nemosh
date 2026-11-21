package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// In bash mode, once nemosh's rc file has run, the session says on one line of stderr which of
// the names bash's file set it changed -- nemosh's has the last word, the user's decision of
// 2026-11-21, and a name it takes from bash's is said rather than lost without a word. A
// variable it only adds to, PATH=$PATH:..., is no change of that kind; nor is a name bash's
// never set. A login session's file is bash's profile.
func TestSession_bashModeSaysWhatNemoshsRCChangedOfBashs(t *testing.T) {
	bashLines := "alias ll='ls -la'\nPS1='b> '\nX=keep\nP=/a\nf() { echo bash; }\n"
	write := func(t *testing.T, home string, files map[string]string) {
		for name, text := range files {
			if err := os.WriteFile(filepath.Join(home, name), []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	nemoshLines := "alias ll='ls -l'\nPS1='n> '\nP=$P:/b\nY=new\nf() { echo nemosh; }\n"
	for _, test := range []struct {
		name, bashFile string
		args           []string
		want           string
	}{
		{name: "interactive", bashFile: ".bashrc", args: []string{"bash", "-i"},
			want: "nemosh: ~/.nemoshrc changed what ~/.bashrc set: PS1, alias ll, function f\n"},
		{name: "login", bashFile: ".bash_profile", args: []string{"bash", "-l", "-i"},
			want: "nemosh: ~/.nemoshrc changed what ~/.bash_profile set: PS1, alias ll, function f\n"},
		{name: "the nemosh name", bashFile: ".bashrc", args: []string{"-i"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			write(t, home, map[string]string{test.bashFile: bashLines, ".nemoshrc": nemoshLines})
			t.Setenv("HOME", home)
			t.Setenv("PS1", "")
			t.Setenv("PS2", "")
			got := runInvocation(t, "", test.args...)
			said := ""
			for _, line := range strings.SplitAfter(got.stderr, "\n") {
				if strings.Contains(line, " changed what ") {
					said += line
				}
			}
			if said != test.want {
				t.Fatalf("said %q, want %q; stderr %q", said, test.want, got.stderr)
			}
		})
	}
}

// Past five names the line says how many more there are, so it stays one line.
func TestOverriddenNotice_staysOneLine(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e", "f", "g"}
	if got, want := overriddenNotice("~/.nemoshrc", "~/.bashrc", names), "nemosh: ~/.nemoshrc changed what ~/.bashrc set: a, b, c, d, e, and 2 more\n"; got != want {
		t.Fatalf("notice %q, want %q", got, want)
	}
}
