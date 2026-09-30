package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bash's --rcfile, --norc and -i -c, each measured in bash 5.3. -i -c is a session running the
// command string: it reads its rc file, an `exit` there ends it, and an error that would end a
// script ends only its line. -i with -c was refused, and so was every long option.
func TestInvocation_sessionCommandAndRCFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return filepath.ToSlash(path)
	}
	rc := write("rc", "echo RCFILE\n")
	exiting := write("exiting", "echo one\nexit 42\necho two\n")
	broken := write("broken", "echo RC; ( echo\n")
	t.Setenv("ENV", write("env", "echo ENV\n"))
	for _, test := range []struct {
		name   string
		args   []string
		stdout string
		status int
	}{
		{name: "the rc file, then the command", args: []string{"--rcfile", rc, "-i", "-c", "echo 2"}, stdout: "RCFILE\n2\n"},
		{name: "--init-file is the same", args: []string{"--init-file", rc, "-i", "-c", "echo 2"}, stdout: "RCFILE\n2\n"},
		{name: "$ENV without --rcfile", args: []string{"-i", "-c", "echo 2"}, stdout: "ENV\n2\n"},
		{name: "--norc reads none", args: []string{"--norc", "-i", "-c", "echo 2"}, stdout: "2\n"},
		{name: "not interactive reads none", args: []string{"--rcfile", rc, "-c", "echo C"}, stdout: "C\n"},
		{name: "exit in the rc file ends it", args: []string{"--rcfile", exiting, "-i", "-c", "echo hello"}, stdout: "one\n", status: 42},
		{name: "a broken rc file is reported and passed", args: []string{"--rcfile", broken, "-i", "-c", "echo flag"}, stdout: "flag\n"},
		{name: "an error ends only its line", args: []string{"--norc", "-i", "-c", "echo $(( 1 / 0 ))\necho one\nexit 42"}, stdout: "one\n", status: 42},
		{name: "interactive in $-", args: []string{"--norc", "-i", "-c", "case $- in *i*) echo yes;; esac"}, stdout: "yes\n"},
		{name: "--noprofile and --login", args: []string{"--noprofile", "--login", "-c", "echo login"}, stdout: "login\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runInvocation(t, "", test.args...)
			if result.stdout != test.stdout || result.status != test.status {
				t.Fatalf("nemosh %q = %q/%d, want %q/%d; stderr %q", test.args, result.stdout, result.status, test.stdout, test.status, result.stderr)
			}
		})
	}
}

// PROMPT_COMMAND runs before each primary prompt, with $? the last command's and $? and $_ as
// they were after it; an error in it ends only it. bash's, measured; busybox has none.
func TestSession_runsPromptCommand(t *testing.T) {
	for _, test := range []struct{ name, stdin, stdout string }{
		{
			name:   "before each prompt",
			stdin:  "PROMPT_COMMAND='echo PROMPT'\necho one\necho two\n",
			stdout: "PROMPT\none\nPROMPT\ntwo\nPROMPT\n",
		},
		{
			name:   "it sees the last status",
			stdin:  "f() { echo last=$?; }\nPROMPT_COMMAND=f\n( exit 42 )\necho ok\n",
			stdout: "last=0\nlast=42\nok\nlast=0\n",
		},
		{
			name:   "an array, each in turn, and $_ and $? kept",
			stdin:  "PROMPT_COMMAND=('echo A' 'echo B; false')\necho x last\necho \"[$_] [$?]\"\n",
			stdout: "A\nB\nx last\nA\nB\n[last] [0]\nA\nB\n",
		},
		{
			name:   "errors end only it",
			stdin:  "PROMPT_COMMAND='echo P $(( 1 / 0 ))'\necho one\nPROMPT_COMMAND=';'\necho two\n",
			stdout: "one\ntwo\n",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runInvocation(t, test.stdin, "--norc", "-i")
			if result.stdout != test.stdout {
				t.Fatalf("stdout %q, want %q; stderr %q", result.stdout, test.stdout, result.stderr)
			}
			if test.name == "errors end only it" && !strings.Contains(result.stderr, "divide by zero") {
				t.Fatalf("stderr %q, want the error reported", result.stderr)
			}
		})
	}
}

// PS0 is written to stderr once a command has been read and before it runs: decoded as a
// prompt and then expanded, bash's order, so an escape that a variable holds stays as it is;
// \# is the command about to run, \! its history number, $? the command before's. Unset or
// empty it writes nothing. bash's, measured in bash 5.3; busybox has none.
func TestSession_writesPS0BeforeEachCommand(t *testing.T) {
	t.Setenv("PS1", "")
	t.Setenv("PS2", "")
	stdin := `PS0='[\# \! $?]'
echo one
false
x='\u'; PS0='[$x]'
echo two
PS0=
echo three
`

	result := runInvocation(t, stdin, "--norc", "-i")

	if result.stdout != "one\ntwo\nthree\n" || result.stderr != `[2 2 0][3 3 0][4 4 1][\u][\u]` {
		t.Fatalf("stdout %q, stderr %q", result.stdout, result.stderr)
	}
}
