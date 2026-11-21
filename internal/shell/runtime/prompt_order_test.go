package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode PS1 and PS2 are decoded and then expanded, bash's order, so what a command
// substitution prints is not decoded again: with g printing C:\new, bash draws PS1='[$(g)]> '
// as [C:\new]> , where busybox -- and nemosh run as nemosh or sh -- decodes the \n it printed
// into a newline. A Windows path in a prompt is all backslashes. Measured 2026-11-21.
func TestPrompt_bashModeDecodesBeforeItExpands(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name string
		bash bool
		want string
	}{
		{name: "nemosh", want: "[C:\new]> "},
		{name: "bash mode", bash: true, want: `[C:\new]> `},
	} {
		t.Run(test.name, func(t *testing.T) {
			rt := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
			if test.bash {
				rt.EnterBashMode()
			}
			if status := rt.RunScript(ctx, `g() { printf %s 'C:\new'; }`+"\n"); status != 0 {
				t.Fatalf("setup exited %d", status)
			}
			if got := rt.Prompt(ctx, `[$(g)]> `, 0); got != test.want {
				t.Errorf("PS1 is drawn %q, want %q", got, test.want)
			}
		})
	}
}

// PS4 is decoded in bash mode too, as bash decodes it: PS4='+\n|$((1+1))| ' traces with a
// newline after the +. busybox only expands it, so \n stays as written there and here.
func TestTrace_bashModeDecodesPS4(t *testing.T) {
	ctx := context.Background()
	for _, test := range []struct {
		name string
		bash bool
		want string
	}{
		{name: "nemosh", want: "+\\n|2| true\n"},
		{name: "bash mode", bash: true, want: "+\n|2| true\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			rt := New(applets.DefaultRegistry, Streams{Stdout: new(bytes.Buffer), Stderr: &stderr})
			if test.bash {
				rt.EnterBashMode()
			}
			rt.RunScript(ctx, "PS4='+\\n|$((1+1))| '\nset -x\ntrue\n")
			if stderr.String() != test.want {
				t.Errorf("the trace is %q, want %q", stderr.String(), test.want)
			}
		})
	}
}
