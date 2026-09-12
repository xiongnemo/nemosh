package runtime_test

import (
	"strings"
	"testing"
)

// Three interactive conveniences that sit either side of one question -- "the word you
// typed is not a command; did you mean a directory?" -- plus the tilde forms that name a
// place on the directory stack.
//
// autocd and CDPATH are both **off unless asked for**, because each changes what a
// *mistake* does. autocd is checked only after command lookup has failed, which is the
// whole safety argument: many trees contain a directory called `test`, and `test` is a
// command.

func TestAutocd(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "a bare directory name changes directory",
			script: "shopt -s autocd\na\npwd",
			want:   "R/a\n",
		},
		{
			// A command of the same name wins, which is why the check happens after
			// the lookup rather than before it.
			name:   "a command still wins",
			script: "mkdir -p echo\nshopt -s autocd\necho hello\npwd",
			want:   "hello\nR\n",
		},
		{
			// `./a` is *not* autocd'd, which surprised me and is what bash does:
			// a path-shaped word is a request to execute that path, and it fails as
			// "is a directory" rather than quietly becoming a cd. Measured, after
			// writing this case the other way round first.
			name:   "a path-shaped word is not autocd",
			script: "shopt -s autocd\n./a 2>/dev/null || echo refused\npwd",
			want:   "refused\nR\n",
		},
		{
			// Off by default, so the ordinary not-found failure is what a typo gets.
			name:   "off unless asked for",
			script: "a 2>/dev/null || echo refused\npwd",
			want:   "refused\nR\n",
		},
		{
			// Only a lone word. `a b` is a command with an argument whatever `a` is,
			// and turning it into a cd would silently drop the argument.
			name:   "only a lone word",
			script: "shopt -s autocd\na b 2>/dev/null || echo refused\npwd",
			want:   "refused\nR\n",
		},
		{
			// A file is not a directory.
			name:   "a regular file is not a directory",
			script: "shopt -s autocd\n: > afile\nafile 2>/dev/null || echo refused",
			want:   "refused\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefix, hide := dirsRoot(t)
			status, stdout, stderr := runSetScript(t, prefix+test.script+"\n")
			if got := hide(stdout); got != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q\n  status %d stderr %q",
					test.script, got, test.want, status, hide(stderr))
			}
		})
	}
}

// CDPATH is POSIX, and is what makes `cd src` work from anywhere once it names the parent.
func TestCDPATH(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{
			name:   "a relative name is searched for",
			script: "CDPATH=ROOT\ncd /tmp\ncd a\npwd",
			want:   "R/a\n",
		},
		{
			// Here wins over CDPATH's answer only because the current directory is
			// tried last -- but it is tried, so an ordinary `cd sub` still works when
			// CDPATH names somewhere else entirely.
			name:   "the current directory is still tried",
			script: "CDPATH=/nowhere-at-all\ncd a\npwd",
			want:   "R/a\n",
		},
		{
			// A path that says where it means is not searched for. Otherwise `cd ./a`
			// could land somewhere else, which is the failure mode that makes people
			// distrust CDPATH.
			name:   "an explicit path is not searched",
			script: "CDPATH=ROOT\ncd /tmp\ncd ./a 2>/dev/null || echo refused",
			want:   "refused\n",
		},
		{
			name:   "unset behaves as before",
			script: "cd a\npwd",
			want:   "R/a\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefix, hide := dirsRoot(t)
			// The prefix is `cd ROOT`, so the root's own path is what ROOT stands for.
			root := strings.TrimSpace(strings.TrimPrefix(prefix, "cd "))
			script := strings.ReplaceAll(test.script, "ROOT", root)
			status, stdout, stderr := runSetScript(t, prefix+script+"\n")
			if got := hide(stdout); got != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q\n  status %d stderr %q",
					test.script, got, test.want, status, hide(stderr))
			}
		})
	}
}

// A CDPATH miss must not narrate itself: the search tries several places and only the last
// one is allowed to complain, or a single `cd nosuch` would print an error per entry.
func TestCDPATH_isQuietWhileSearching(t *testing.T) {
	prefix, hide := dirsRoot(t)
	root := strings.TrimSpace(strings.TrimPrefix(prefix, "cd "))
	status, _, stderr := runSetScript(t,
		prefix+"CDPATH="+root+":/nowhere:/also-nowhere\ncd nosuchdir\n")
	if status == 0 {
		t.Fatal("cd into a missing directory succeeded")
	}
	if count := strings.Count(hide(stderr), "cd:"); count != 1 {
		t.Fatalf("the search reported %d times, want 1:\n%s", count, hide(stderr))
	}
}

// `~+`, `~-`, `~N`, `~+N` and `~-N` name a place on the stack. Measured against bash.
func TestTilde_directoryForms(t *testing.T) {
	for _, test := range []struct {
		name   string
		script string
		want   string
	}{
		{name: "~+ is here", script: "cd a\necho ~+", want: "R/a\n"},
		{name: "~- is where I was", script: "cd a\ncd ../b\necho ~-", want: "R/a\n"},
		{name: "~- carries a path", script: "cd a\ncd ../b\necho ~-/sub", want: "R/a/sub\n"},
		{name: "~0 is the top", script: "pushd a >/dev/null\necho ~0", want: "R/a\n"},
		{name: "~1 is under it", script: "pushd a >/dev/null\necho ~1", want: "R\n"},
		{name: "~-0 counts from the far end", script: "pushd a >/dev/null\necho ~-0", want: "R\n"},

		// What must *not* expand. `~user` is a measured non-goal, and a tilde that is
		// not at the start of a word was never a reference.
		{name: "a user name is left alone", script: "echo ~nosuchuser", want: "~nosuchuser\n"},
		{name: "a tilde inside a word", script: "echo a~+b", want: "a~+b\n"},
		{name: "quoted is literal", script: "echo \"~+\"", want: "~+\n"},
		// An index past the end is not a directory, so it stays as written rather than
		// becoming the empty string -- an empty path silently means "here".
		{name: "an index past the end", script: "echo ~9", want: "~9\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefix, hide := dirsRoot(t)
			status, stdout, stderr := runSetScript(t, prefix+test.script+"\n")
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, hide(stderr))
			}
			if got := hide(stdout); got != test.want {
				t.Fatalf("%s\n  got  %q\n  want %q", test.script, got, test.want)
			}
		})
	}
}

// `~-` before any `cd` is left as written rather than becoming empty, because an empty
// path silently means the current directory and would move a file somewhere nobody asked.
func TestTilde_previousDirectoryBeforeAnyCd(t *testing.T) {
	prefix, hide := dirsRoot(t)
	status, stdout, _ := runSetScript(t, prefix+"unset OLDPWD\necho ~-\n")
	if status != 0 {
		t.Fatalf("status = %d", status)
	}
	if got := hide(stdout); got != "~-\n" {
		t.Fatalf("got %q, want %q", got, "~-\n")
	}
}
