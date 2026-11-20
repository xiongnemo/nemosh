package runtime

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// sh and bash that PATH has no program for are this shell: `sh -c` runs, `bash -c` runs, and
// `command -v` names the binary that runs them. Each was `not found`, 127, as any other name
// PATH has nothing for still is.
func TestShellFallback_runsThisShellForShAndBash(t *testing.T) {
	self, err := jobExecutable()
	if err != nil {
		t.Skip("no binary of this shell to fall back to")
	}
	t.Setenv("PATH", t.TempDir())
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(),
		"sh -c 'exit 33'; echo \"sh $?\"\nbash -c 'echo bash $((1+2))'\ncommand -v sh\nnemosh-no-such-fallback 2>/dev/null; echo \"other $?\"\n")

	// Then
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if status != 0 || len(lines) != 4 || lines[0] != "sh 33" || lines[1] != "bash 3" || lines[3] != "other 127" ||
		!strings.EqualFold(filepath.ToSlash(lines[2]), filepath.ToSlash(self)) {
		t.Fatalf("status %d, stdout %q, stderr %q; want sh 33, bash 3, %s and other 127", status, stdout.String(), stderr.String(), self)
	}
}

// sh and bash are this shell when PATH has a program by either name too, as busybox-w32's
// applets of those names are: Git's bash, or WSL's System32\bash.exe, ran instead, and WSL's ran
// the script in Linux. Run as bash it says so, in bash 5.2's words, and run as sh it does not.
// NEMOSH_OVERRIDE_APPLETS puts PATH's first again.
func TestShellNames_runThisShellBeforePath(t *testing.T) {
	self, err := jobExecutable()
	if err != nil {
		t.Skip("no binary of this shell to run")
	}
	dir := t.TempDir()
	for _, name := range []string{"bash", "sh"} {
		if err := os.WriteFile(filepath.Join(dir, name+".bat"), []byte("@echo path-"+name+" %*\r\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), `bash -c 'echo "$0 $BASH_VERSION ${BASH_VERSINFO[*]}"'
sh -c 'echo "$0 ${BASH_VERSION-unset}"'
command -v bash
which -a bash
NEMOSH_OVERRIDE_APPLETS=bash
bash one
sh -c 'echo "$0 ${BASH_VERSION-unset}"'
`)

	// Then
	machine := map[string]string{"amd64": "x86_64", "386": "i686", "arm64": "aarch64"}[goruntime.GOARCH]
	want := "bash 5.2.37(1)-release 5 2 37 1 release " + machine + "-pc-msys\n" +
		"sh unset\n" +
		filepath.ToSlash(self) + "\n" +
		filepath.ToSlash(self) + "\n" + filepath.ToSlash(filepath.Join(dir, "bash.bat")) + "\n" +
		"path-bash one\n" +
		"sh unset\n"
	got := strings.ReplaceAll(stdout.String(), "\r\n", "\n")
	if status != 0 || !strings.EqualFold(got, want) {
		t.Fatalf("status %d, stdout %q, stderr %q; want %q", status, got, stderr.String(), want)
	}
}

// The guard a bash script opens with, `[ -z "$BASH_VERSION" ] && exec bash "$0" "$@"`, runs it
// again under bash once, and then the script goes on: bash is this shell, and says so. Were
// BASH_VERSION unset under bash too, the guard would run the script again for ever.
func TestShellNames_bashGuardRunsTheScriptOnce(t *testing.T) {
	if _, err := jobExecutable(); err != nil {
		t.Skip("no binary of this shell to run")
	}
	dir := t.TempDir()
	guard := filepath.Join(dir, "guard.sh")
	script := "GUARD_DEPTH=$((GUARD_DEPTH+1)); export GUARD_DEPTH\n" +
		"[ \"$GUARD_DEPTH\" -gt 3 ] && { echo looped; exit 9; }\n" +
		"[ -z \"$BASH_VERSION\" ] && exec bash \"$0\" \"$@\"\n" +
		"echo \"ran $GUARD_DEPTH ${BASH_VERSINFO[0]}.${BASH_VERSINFO[1]} $1\"\n"
	if err := os.WriteFile(guard, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("GUARD_DEPTH", "0")
	var stdout, stderr bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})

	// When
	status := rt.RunScript(context.Background(), "sh '"+filepath.ToSlash(guard)+"' one\n")

	// Then
	if status != 0 || stdout.String() != "ran 2 5.2 one\n" {
		t.Fatalf("status %d, stdout %q, stderr %q; want ran 2 5.2 one", status, stdout.String(), stderr.String())
	}
}
