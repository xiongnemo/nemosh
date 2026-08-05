package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func TestParseScript_marksFunctionDefinitionBackgroundWithoutExecutingBody(t *testing.T) {
	// Given / When
	script, err := ParseScript("f() { echo body; } &\necho after\n")

	// Then
	if err != nil {
		t.Fatal(err)
	}
	marked, ok := script.program[0].(backgroundNode)
	if !ok {
		t.Fatalf("node = %T, want backgroundNode", script.program[0])
	}
	if _, ok := marked.value.(functionDefinition); !ok {
		t.Fatalf("background value = %T, want functionDefinition", marked.value)
	}
	var stdout bytes.Buffer
	rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout})
	status, _ := rt.executePrepared(context.Background(), script)
	rt.jobScope.drain()
	if status != 0 || stdout.String() != "after\n" || len(rt.functions) != 0 {
		t.Fatalf("status = %d, stdout = %q, functions = %#v", status, stdout.String(), rt.functions)
	}
}

func TestParseScript_marksMultilineCompoundsBackgroundAsTypedNodes(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   any
	}{
		{name: "if", source: "if true\nthen\necho if-body\nfi &\necho after\n", want: ifNode{}},
		{name: "while", source: "while false\ndo\necho loop-body\ndone &\necho after\n", want: loopNode{}},
		{name: "case", source: "case x in\nx)\necho case-body\n;;\nesac &\necho after\n", want: caseNode{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			script, err := ParseScript(test.source)
			if err != nil {
				t.Fatal(err)
			}
			marked, ok := script.program[0].(backgroundNode)
			if !ok {
				t.Fatalf("node = %T, want backgroundNode", script.program[0])
			}
			switch test.want.(type) {
			case ifNode:
				_, ok = marked.value.(ifNode)
			case loopNode:
				_, ok = marked.value.(loopNode)
			case caseNode:
				_, ok = marked.value.(caseNode)
			}
			if !ok {
				t.Fatalf("background value = %T, want %T", marked.value, test.want)
			}
		})
	}
}
