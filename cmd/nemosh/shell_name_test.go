package main

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"testing"
)

// `nemosh bash ARGS` and a nemosh named bash.exe are bash reading ARGS, as `busybox bash` and
// busybox-w32 named bash are its ash: $0 is bash, and BASH_VERSION and the read-only
// BASH_VERSINFO are bash 5.2's. In `nemosh bash ARGS` an operand that is an applet's name is a
// script all the same. sh is the same shell without them. That is what a command named bash or
// sh runs; see the runtime's shell_fallback.go.
func TestRun_bashAndShAreThisShellByName(t *testing.T) {
	machine := map[string]string{"amd64": "x86_64", "386": "i686", "arm64": "aarch64"}[goruntime.GOARCH]
	version := "5.2.37(1)-release 5 2 37 1 release " + machine + "-pc-msys"
	report := `echo "$0 ${BASH_VERSION-unset} ${BASH_VERSINFO[*]-}"`
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		{name: "nemosh bash -c", args: []string{"nemosh", "bash", "-c", report}, want: "bash " + version + "\n"},
		{name: "bash.exe -c", args: []string{`C:\bin\bash.exe`, "-c", report}, want: "bash " + version + "\n"},
		{name: "nemosh sh -c", args: []string{"nemosh", "sh", "-c", report}, want: "sh unset \n"},
		// What the shell launches is SELF NAME, whatever SELF is called: a nemosh named
		// bash.exe, as Oils' harness installs it, runs a command named sh or bash, and a
		// #!/bin/cat, as any other does.
		{name: "bash.exe bash -c", args: []string{`C:\bin\bash.exe`, "bash", "-c", report}, want: "bash " + version + "\n"},
		{name: "bash.exe sh -c", args: []string{`C:\bin\bash.exe`, "sh", "-c", report}, want: "sh unset \n"},
		{name: "bash.exe APPLET", args: []string{`C:\bin\bash.exe`, "echo", "an applet"}, want: "an applet\n"},
		{name: "nemosh -c", args: []string{"nemosh", "-c", report}, want: "nemosh unset \n"},
		{name: "declared as bash declares them", args: []string{"nemosh", "bash", "-c", "declare -p BASH_VERSINFO BASH_VERSION"},
			want: `declare -ar BASH_VERSINFO=([0]="5" [1]="2" [2]="37" [3]="1" [4]="release" [5]="` + machine + `-pc-msys")` + "\n" +
				`declare -- BASH_VERSION="5.2.37(1)-release"` + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := runArgs(t, "", test.args...)
			if got.status != 0 || got.stdout != test.want {
				t.Fatalf("status %d, stdout %q, stderr %q; want %q", got.status, got.stdout, got.stderr, test.want)
			}
		})
	}

	// `bash --help` is how Oils' sh-usage asks a shell whether it obeys --help, and what a
	// command named bash launches is `nemosh bash --help`. It was an invalid option.
	t.Run("bash --help and --version", func(t *testing.T) {
		for _, option := range []string{"--help", "--version"} {
			got := runArgs(t, "", "nemosh", "bash", option)
			if got.status != 0 || got.stdout == "" {
				t.Errorf("nemosh bash %s: status %d, stdout %q, stderr %q; want it printed", option, got.status, got.stdout, got.stderr)
			}
		}
	})

	t.Run("an applet's name is a script", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "echo"), []byte("echo from-the-script $1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		got := runArgs(t, dir, "nemosh", "bash", "echo", "one")
		if got.status != 0 || got.stdout != "from-the-script one\n" {
			t.Fatalf("status %d, stdout %q, stderr %q; want from-the-script one", got.status, got.stdout, got.stderr)
		}
	})
}
