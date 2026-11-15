package runtime_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// command_not_found_handle is bash's: defined, it runs in place of the "not found", in a
// subshell, given the name and its arguments, and what it returns is the command's status.
// A name with a slash is a path, not a lookup, and does not reach it. Measured in bash 5.3.
func TestRuntime_runsCommandNotFoundHandle(t *testing.T) {
	status, stdout, stderr := runSetScript(t, "command_not_found_handle() { echo \"handled [$1] $#\"; x=changed; return 5; }\n"+
		"x=orig\nnosuchcmd a b\necho \"status $? x=$x\"\n./nosuch/path\necho \"path status $?\"\n")
	if status != 0 || stdout != "handled [nosuchcmd] 3\nstatus 5 x=orig\npath status 127\n" {
		t.Fatalf("status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
	if !strings.Contains(stderr, "./nosuch/path: not found") {
		t.Fatalf("stderr = %q, want the path reported as not found", stderr)
	}
}

// A command the handler cannot find is reported, not handed back to the handler. bash hands
// it back, a subshell deeper each time, and printed some thousand lines before it stopped.
func TestRuntime_doesNotRecurseIntoCommandNotFoundHandle(t *testing.T) {
	status, stdout, stderr := runSetScript(t, "command_not_found_handle() { nosuchinner; echo \"inner $?\"; return 3; }\nnosuchouter\necho \"outer $?\"\n")
	if status != 0 || stdout != "inner 127\nouter 3\n" || !strings.Contains(stderr, "nosuchinner: not found") {
		t.Fatalf("status %d, stdout %q, stderr %q", status, stdout, stderr)
	}
}

// At a prompt a name that is not found is answered with the names it is nearly, from what
// this shell can run. Not in a script, whose output has to say the same on every machine.
func TestRuntime_suggestsANearName_atAPromptOnly(t *testing.T) {
	script := "greeting() { :; }\ngreetign\n"
	_, _, stderr := runSetScript(t, script)
	if strings.Contains(stderr, "did you mean") {
		t.Fatalf("a script was offered a suggestion: %q", stderr)
	}

	var out bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &out, Stderr: &out})
	rt.MarkSession()
	rt.RunScript(context.Background(), script)
	if !strings.Contains(out.String(), "greetign: not found") || !strings.Contains(out.String(), "did you mean `greeting`?") {
		t.Fatalf("output = %q, want greeting suggested", out.String())
	}
}

// A PowerShell script is named as one, by its name on PATH or by its path, with how to run it:
// this shell does not start PowerShell for it, and "not found" for a file that is there, or
// "not something Windows will start", said nothing about what to do.
func TestRuntime_namesAPowerShellScript(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "deploy.ps1")
	if err := os.WriteFile(script, []byte("Write-Output hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"PATH='" + filepath.ToSlash(directory) + "'\ndeploy\n", "'" + filepath.ToSlash(script) + "'\n"} {
		_, _, stderr := runSetScript(t, line)
		if !strings.Contains(stderr, "PowerShell") || !strings.Contains(stderr, "powershell -File") {
			t.Errorf("%q: stderr = %q, want the PowerShell hint", line, stderr)
		}
	}
}
