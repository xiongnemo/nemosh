package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Ubuntu's stock startup files, as testdata/README.md says where from, run as a session's own:
// what an administrator gets by copying ~/.bashrc into ~/.nemoshrc, or starting `bash -l` with
// Ubuntu's ~/.profile. Each runs to the end and says nothing, and leaves what it sets. Before
// 2026-11-21 /etc/bash.bashrc closed the session at its first line, `[ -z "${PS1-}" ] && return`.
func TestSession_runsUbuntusStockStartupFiles(t *testing.T) {
	// Their prompts and colours turn on TERM, which a Windows console does not set.
	t.Setenv("TERM", "")
	os.Unsetenv("TERM")
	ubuntuPS1 := `${debian_chroot:+($debian_chroot)}\u@\h:\w\$ `
	fixture := func(name string) string {
		path, err := filepath.Abs(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(path)
	}

	t.Run("skel.bashrc as the rc file", func(t *testing.T) {
		report := `echo "$HISTCONTROL $HISTSIZE $HISTFILESIZE"; echo "$PS1"
alias ll la l alert >/dev/null && echo aliases
shopt -q histappend && shopt -q checkwinsize && echo shopts
echo "${debian_chroot-no chroot} ${color_prompt-no colour}"`
		result := runInvocation(t, "", "--rcfile", fixture("ubuntu-skel.bashrc"), "-i", "-c", report)
		want := "ignoreboth 1000 2000\n" + ubuntuPS1 + "\naliases\nshopts\nno chroot no colour\n"
		if result.stdout != want || result.stderr != "" || result.status != 0 {
			t.Fatalf("stdout %q, stderr %q, status %d; want %q", result.stdout, result.stderr, result.status, want)
		}
	})

	t.Run("/etc/bash.bashrc as the rc file", func(t *testing.T) {
		result := runInvocation(t, "", "--rcfile", fixture("ubuntu-etc-bash.bashrc"), "-i", "-c", `echo reached; echo "$PS1"`)
		want := "reached\n" + ubuntuPS1 + "\n"
		if result.stdout != want || result.stderr != "" || result.status != 0 {
			t.Fatalf("stdout %q, stderr %q, status %d; want %q", result.stdout, result.stderr, result.status, want)
		}
	})

	// ~/.profile reads ~/.bashrc when BASH_VERSION is set, which it is when this shell is bash.
	t.Run("bash -l with skel.profile and skel.bashrc", func(t *testing.T) {
		home := t.TempDir()
		for name, from := range map[string]string{".profile": "ubuntu-skel.profile", ".bashrc": "ubuntu-skel.bashrc"} {
			data, err := os.ReadFile(filepath.Join("testdata", from))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, name), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Setenv("HOME", home)
		result := runInvocation(t, "", "bash", "-l", "-i", "-c", `echo "$HISTCONTROL $PS1"`)
		want := "ignoreboth " + ubuntuPS1 + "\n"
		if result.stdout != want || result.stderr != "" || result.status != 0 {
			t.Fatalf("stdout %q, stderr %q, status %d; want %q", result.stdout, result.stderr, result.status, want)
		}
	})
}
