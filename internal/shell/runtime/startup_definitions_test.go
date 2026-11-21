package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// Definitions names what a startup file can define as a person names it, each with the text
// that defines it, so a session can tell what one file changed of what another set. What the
// shell keeps for itself as it runs -- $_, $PIPESTATUS -- is not a definition.
func TestDefinitions_nameWhatAStartupFileCanDefine(t *testing.T) {
	rt := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	before := rt.Definitions()
	script := `PS1='p> '
a=(1 2)
declare -A m=([k]=v)
alias ll='ls -l'
f() { :; }
set -o vi
shopt -s nullglob
complete -W 'x y' git
true | false
`
	if status := rt.RunScript(context.Background(), script); status != 1 {
		t.Fatalf("setup exited %d", status)
	}
	after := rt.Definitions()
	for _, name := range []string{"PS1", "a", "m", "alias ll", "function f", "set -o vi", "shopt nullglob", "complete git"} {
		if after[name] == "" || after[name] == before[name] {
			t.Errorf("%q is %q after the script and %q before it; want it defined, and changed", name, after[name], before[name])
		}
	}
	for _, name := range []string{"_", "PIPESTATUS"} {
		if _, there := after[name]; there {
			t.Errorf("%q is a definition", name)
		}
	}
}
