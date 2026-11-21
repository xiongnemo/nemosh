package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// Every readline function the runtime says this editor carries out is a key here, and each key
// it lists for the function before any bind is decoded by the editor to that same key: the
// list `bind -p` prints cannot drift from what the keys do.
func TestBind_everyFunctionIsAKeyOfTheEditor(t *testing.T) {
	for _, name := range runtime.ReadlineActionNames() {
		want, ok := functionKey(name, "a")
		if !ok {
			t.Errorf("%s is listed as carried out, and the editor has no key for it", name)
			continue
		}
		for _, notation := range runtime.ReadlineActionKeys(name) {
			keys, err := runtime.ParseKeyseq(notation)
			if err != nil {
				t.Errorf("%s: %q: %v", name, notation, err)
				continue
			}
			got, consumed := decodeKey(keys)
			if consumed != len(keys) || got.kind != want.kind || got.value != want.value {
				t.Errorf("%q decodes to %+v, %d bytes; %s is %+v", notation, got, consumed, name, want)
			}
		}
	}
}

// Keys bind -x bound run the command over the line, which then reads as the command left it,
// the cursor where it put READLINE_POINT -- fzf's Ctrl-T so puts in the file picked. The
// terminal is given back while the command runs.
func TestBind_xRunsTheCommandOverTheLine(t *testing.T) {
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	ctx := context.Background()
	script := `bind -x '"\C-t": READLINE_LINE="${READLINE_LINE:0:READLINE_POINT}picked ${READLINE_LINE:READLINE_POINT}"; READLINE_POINT=$((READLINE_POINT + 7))'` + "\n"
	if status := rt.RunScript(ctx, script); status != 0 {
		t.Fatalf("bind -x exited %d", status)
	}
	editor := newLineEditor(strings.NewReader("vi \x14x\r"), new(bytes.Buffer), t.TempDir())
	editor.bindings = rt.MatchKeyBinding
	suspended := 0
	editor.suspend = func(run func()) { suspended++; run() }
	editor.shellCommand = func(command, line string, point int) (string, int) {
		return rt.RunKeyCommand(ctx, command, line, point)
	}
	line, err := editor.readLine(ctx, "$ ")
	if err != nil || line != "vi picked x" || suspended != 1 {
		t.Fatalf("line %q, %v, terminal given back %d times; want \"vi picked x\" and once", line, err, suspended)
	}
}

// A sequence bound to a function is that function's key; one bound to a macro types the macro;
// one bound as the start of a longer one waits for the next key. The bindings are a session's,
// set by the bind builtin.
func TestBind_theEditorReadsKeysThroughWhatBindSet(t *testing.T) {
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: new(bytes.Buffer), Stderr: new(bytes.Buffer)})
	script := "bind '\"\\C-o\": beginning-of-line'\nbind '\"\\C-x\\C-y\": \"typed \"'\nbind '\"zz\": kill-line'\n"
	if status := rt.RunScript(context.Background(), script); status != 0 {
		t.Fatalf("bind exited %d", status)
	}
	for _, test := range []struct{ name, keys, want string }{
		{name: "a function", keys: "echo\x0fsay \r", want: "say echo"},
		{name: "a macro", keys: "echo \x18\x19x\r", want: "echo typed x"},
		{name: "the start of a longer sequence", keys: "abc\x01zz\r", want: ""},
		{name: "a key that only begins one", keys: "az\r", want: "az"},
	} {
		t.Run(test.name, func(t *testing.T) {
			editor := newLineEditor(strings.NewReader(test.keys), new(bytes.Buffer), t.TempDir())
			editor.bindings = rt.MatchKeyBinding
			line, err := editor.readLine(context.Background(), "$ ")
			if err != nil || line != test.want {
				t.Fatalf("line %q, %v; want %q", line, err, test.want)
			}
		})
	}
}
