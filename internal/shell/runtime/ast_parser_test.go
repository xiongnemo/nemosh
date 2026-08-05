package runtime

import (
	"errors"
	"testing"
)

func TestParseScript_preservesOrderedWordParts(t *testing.T) {
	script, err := ParseScript(`echo a'b'"$value"\$x$(echo nested) ''`)
	if err != nil {
		t.Fatal(err)
	}
	command := firstSimpleCommand(t, script)
	parts := command.words[1].parts
	want := []wordPartKind{wordPartLiteral, wordPartLiteral, wordPartParameter, wordPartEscaped, wordPartLiteral, wordPartCommandSubstitution}
	if len(parts) != len(want) {
		t.Fatalf("parts = %#v, want %d parts", parts, len(want))
	}
	for index := range want {
		if parts[index].kind != want[index] {
			t.Fatalf("part %d kind = %v, want %v", index, parts[index].kind, want[index])
		}
	}
	if !command.words[2].quotedEmpty {
		t.Fatal("explicit quoted empty word was not preserved")
	}
}

func TestParseScript_preservesRedirectOrderAndTypedLists(t *testing.T) {
	script, err := ParseScript("a >out 2>&1 | b && c || d\n")
	if err != nil {
		t.Fatal(err)
	}
	parsed := script.program[0].(listNode).value
	andOr := parsed.items[0].value
	if len(andOr.pipelines) != 3 || len(andOr.operators) != 2 {
		t.Fatalf("and-or = %#v", andOr)
	}
	first := andOr.pipelines[0]
	if len(first.commands) != 2 {
		t.Fatalf("pipeline commands = %d, want 2", len(first.commands))
	}
	redirects := first.commands[0].(simpleCommand).redirects
	if len(redirects) != 2 || redirects[0].kind != redirectOutput || redirects[1].kind != redirectDup {
		t.Fatalf("redirects = %#v", redirects)
	}
}

func TestParseScript_classifiesMalformedAndIncomplete(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		incomplete bool
	}{
		{name: "open quote", source: `echo "`, incomplete: true},
		{name: "pipeline rhs", source: "echo x |", incomplete: true},
		{name: "redirect operand", source: "echo x >", incomplete: true},
		{name: "leading pipeline", source: "| echo x", incomplete: false},
		{name: "interior empty pipeline", source: "a | | b", incomplete: false},
		{name: "triple pipeline", source: "a ||| b", incomplete: false},
		{name: "unexpected closer", source: "fi\n", incomplete: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseScript(test.source)
			if err == nil {
				t.Fatal("ParseScript() error = nil")
			}
			if got := errors.Is(err, ErrIncompleteScript); got != test.incomplete {
				t.Fatalf("incomplete = %v, want %v; err = %v", got, test.incomplete, err)
			}
		})
	}
}

func TestParseScript_preservesMixedTopLevelOrderAsTypedNodes(t *testing.T) {
	script, err := ParseScript("echo before\nif true\nthen\necho inside\nfi\necho after\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(script.program) != 3 {
		t.Fatalf("program nodes = %d, want 3", len(script.program))
	}
	if _, ok := script.program[0].(listNode); !ok {
		t.Fatalf("node 0 = %T, want listNode", script.program[0])
	}
	if _, ok := script.program[1].(ifNode); !ok {
		t.Fatalf("node 1 = %T, want ifNode", script.program[1])
	}
	if _, ok := script.program[2].(listNode); !ok {
		t.Fatalf("node 2 = %T, want listNode", script.program[2])
	}
}

func TestParseScript_rejectsMalformedNestedCommandSubstitution(t *testing.T) {
	_, err := ParseScript("echo prefix\necho $(if true)\n")
	if err == nil {
		t.Fatal("ParseScript() error = nil")
	}
}

func firstSimpleCommand(t *testing.T, script Script) simpleCommand {
	t.Helper()
	return script.program[0].(listNode).value.items[0].value.pipelines[0].commands[0].(simpleCommand)
}
