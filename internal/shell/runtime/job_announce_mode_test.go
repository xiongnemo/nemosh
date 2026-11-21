package runtime

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A session says `[1] PID` for a job its own command line puts in the background. bash says
// nothing for one a subshell, a command substitution or a pipeline stage does, where the shell
// is not the session's -- atuin's `(atuin ... &)` before each prompt printed a job number every
// time -- and busybox-w32 says it there as well. bash mode takes bash's, and nemosh and sh keep
// busybox's. Measured 2026-11-21 with Git for Windows' bash 5.3 and busybox-w32, both -i.
func TestJobs_bashModeAnnouncesOnlyTheSessionsOwnJobs(t *testing.T) {
	for _, test := range []struct {
		name, script string
		inSubshell   bool
	}{
		{name: "at the prompt", script: "true &\nwait\n"},
		{name: "in a subshell", script: "(true &)\nwait\n", inSubshell: true},
		{name: "in a command substitution", script: "x=$(true &)\nwait\n", inSubshell: true},
		{name: "in a group in a pipeline", script: "{ true & } | cat\nwait\n", inSubshell: true},
	} {
		for _, bash := range []bool{false, true} {
			mode := " as nemosh"
			if bash {
				mode = " in bash mode"
			}
			t.Run(test.name+mode, func(t *testing.T) {
				var stdout, stderr bytes.Buffer
				rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
				if bash {
					rt.EnterBashMode()
				}
				rt.MarkSession()
				rt.RunScript(context.Background(), test.script)
				announced := !(bash && test.inSubshell)
				if said := stderr.String(); strings.HasPrefix(said, "[1] ") != announced || (!announced && said != "") {
					t.Fatalf("stderr %q, announced is to be %v", said, announced)
				}
			})
		}
	}
}
