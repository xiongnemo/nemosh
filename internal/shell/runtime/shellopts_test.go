package runtime_test

import "testing"

// SHELLOPTS and BASHOPTS are bash's lists of the `set -o` and shopt names that are on,
// joined by colons and kept current, and both are read-only. busybox has neither; both were
// unset, and `${SHELLOPTS:?}` ended the script.
func TestShellopts(t *testing.T) {
	tests := []struct {
		name, script, stdout string
		status               int
	}{
		{
			name:   "what is on",
			script: "echo \"$SHELLOPTS\"\nset -o pipefail -u\necho \"$SHELLOPTS\"\n",
			stdout: "braceexpand:hashall:igncr:interactive-comments\nbraceexpand:hashall:igncr:interactive-comments:nounset:pipefail\n",
		},
		{
			name:   "shopt's",
			script: "shopt -u sourcepath promptvars; shopt -s nullglob\necho \"$BASHOPTS\"\n",
			stdout: "checkwinsize:cmdhist:complete_fullquote:expand_aliases:extglob:force_fignore:globasciiranges:" +
				"globskipdots:hostcomplete:interactive_comments:localvar_unset:nullglob:progcomp\n",
		},
		{
			name:   "both are set",
			script: "test -v SHELLOPTS && test -v BASHOPTS && echo set\n",
			stdout: "set\n",
		},
		{
			name:   "read-only",
			script: "SHELLOPTS=x\necho after\n",
			stdout: "", status: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			status, stdout, stderr := runSetScript(t, test.script)

			// Then
			if stdout != test.stdout || status != test.status {
				t.Fatalf("got %q/%d, want %q/%d; stderr = %q", stdout, status, test.stdout, test.status, stderr)
			}
		})
	}
}

// Exported, SHELLOPTS goes to a child with its value at the time, and a shell that inherits
// one turns on what it names -- a name it has not got is passed over -- and keeps it
// exported, as bash does.
func TestShellopts_crossesToAChild(t *testing.T) {
	t.Run("exported", func(t *testing.T) {
		status, stdout, stderr := runSetScript(t, "export SHELLOPTS\nset -o pipefail\nenv | grep '^SHELLOPTS='\n")
		if want := "SHELLOPTS=braceexpand:hashall:igncr:interactive-comments:pipefail\n"; stdout != want || status != 0 {
			t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
		}
	})
	t.Run("inherited", func(t *testing.T) {
		t.Setenv("SHELLOPTS", "nounset:nosuch:pipefail")
		t.Setenv("BASHOPTS", "nullglob")
		status, stdout, stderr := runSetScript(t,
			"echo \"$SHELLOPTS\"\nshopt -p nullglob\nenv | grep -c '^SHELLOPTS=braceexpand'\n")
		want := "braceexpand:hashall:igncr:interactive-comments:nounset:pipefail\nshopt -s nullglob\n1\n"
		if stdout != want || status != 0 {
			t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
		}
	})
}
