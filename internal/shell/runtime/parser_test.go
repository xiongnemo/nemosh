package runtime

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseScript_normalizesLogicalLines(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{name: "empty input", source: "", want: nil},
		{name: "blank and comment input", source: " \r\n\t\r\n # comment\r\n", want: nil},
		{name: "CRLF without final newline", source: "echo one\r\necho two", want: []string{"echo one", "echo two"}},
		{name: "comment boundary", source: "echo '# kept' # removed\necho foo#bar\n", want: []string{"echo '# kept'", "echo foo#bar"}},
		{name: "continued line", source: "echo one\\\ntwo\n", want: []string{"echo onetwo"}},
		{name: "single quoted backslash", source: "echo 'one\\two'\n", want: []string{"echo 'one\\two'"}},
		{name: "multiline single quote", source: "printf '%s' 'one\ntwo'\n", want: []string{"printf '%s' 'one\ntwo'"}},
		{name: "multiline double quote", source: "printf '%s' \"one\ntwo\"\n", want: []string{"printf '%s' \"one\ntwo\""}},
		{name: "multiline command substitution", source: "echo $(printf '%s' \"one\ntwo\")\n", want: []string{"echo $(printf '%s' \"one\ntwo\")"}},
		{name: "nested substitution with comments", source: "echo $(echo '# kept'\necho $(printf '%s' \"one\ntwo\")) # removed\n", want: []string{"echo $(echo '# kept'\necho $(printf '%s' \"one\ntwo\"))"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			lines, err := logicalLines(tt.source)

			// Then
			if err != nil {
				t.Fatalf("ParseScript() error = %v", err)
			}
			if !reflect.DeepEqual(lines, tt.want) {
				t.Fatalf("logicalLines() = %#v, want %#v", lines, tt.want)
			}
		})
	}
}

func TestParseScript_reportsIncompleteLexicalConstructs(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "single quote", source: "echo 'open"},
		{name: "double quote", source: "echo \"open"},
		{name: "command substitution", source: "echo $(echo $(echo hi)) $(echo"},
		{name: "continued line", source: "echo value\\\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			_, err := ParseScript(tt.source)

			// Then
			if !errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want ErrIncompleteScript", err)
			}
		})
	}
}

func TestParseScript_recordsNestedCompoundSpans(t *testing.T) {
	// Given
	source := "if true\nthen\nfor x in one\ndo\ncase $x in\none)\necho one\n;;\nesac\ndone\nfi\n"

	// When
	script, err := ParseScript(source)

	// Then
	if err != nil {
		t.Fatalf("ParseScript() error = %v", err)
	}
	root, ok := script.program[0].(ifNode)
	if !ok || len(root.thenBody) != 1 {
		t.Fatalf("program = %#v, want nested if body", script.program)
	}
	loop, ok := root.thenBody[0].(loopNode)
	if !ok || len(loop.body) != 1 {
		t.Fatalf("if body = %#v, want nested loop", root.thenBody)
	}
	if _, ok := loop.body[0].(caseNode); !ok {
		t.Fatalf("loop body node = %T, want caseNode", loop.body[0])
	}
}

func TestParseScript_ordersSequentialCompoundSpansByAbsoluteLine(t *testing.T) {
	// Given
	source := "if true\nthen\necho one\nfi\nwhile false\ndo\necho two\ndone\n"

	// When
	script, err := ParseScript(source)

	// Then
	if err != nil {
		t.Fatalf("ParseScript() error = %v", err)
	}
	if len(script.program) != 2 {
		t.Fatalf("program nodes = %d, want 2", len(script.program))
	}
	if _, ok := script.program[0].(ifNode); !ok {
		t.Fatalf("node 0 = %T, want ifNode", script.program[0])
	}
	if _, ok := script.program[1].(loopNode); !ok {
		t.Fatalf("node 1 = %T, want loopNode", script.program[1])
	}
}

func TestParseScript_reportsIncompleteCompounds(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "if", source: "if true\nthen\necho yes\n"},
		{name: "loop before do", source: "while true\n"},
		{name: "loop after do", source: "until false\ndo\necho yes\n"},
		{name: "case", source: "case x in\nx)\necho x\n;;\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			_, err := ParseScript(tt.source)

			// Then
			if !errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want ErrIncompleteScript", err)
			}
		})
	}
}

func TestParseScript_rejectsUnexpectedOrMisorderedClosers(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "unexpected fi", source: "fi\n"},
		{name: "unexpected done", source: "done\n"},
		{name: "unexpected esac", source: "esac\n"},
		{name: "do closing if", source: "if true\ndo\n"},
		{name: "fi closing loop", source: "for x in one\ndo\nfi\n"},
		{name: "done before do", source: "while true\ndone\n"},
		{name: "fi before then", source: "if true\nfi\n"},
		{name: "else before then", source: "if true\nelse\nfi\n"},
		{name: "duplicate do", source: "while true\ndo\ndo\ndone\n"},
		{name: "case text before first arm", source: "case x in\necho stray\nesac\n"},
		{name: "case text between arms", source: "case x in\nx)\necho x\n;;\necho stray\ny)\necho y\n;;\nesac\n"},
		{name: "case text before esac", source: "case x in\nx)\necho x\n;;\necho stray\nesac\n"},
		{name: "empty case pattern", source: "case x in\n)\necho empty\n;;\nesac\n"},
		{name: "outer case text after nested case", source: "case outer in\nouter)\ncase inner in\ninner)\necho inner\n;;\nesac\n;;\necho stray\nesac\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			_, err := ParseScript(tt.source)

			// Then
			if err == nil {
				t.Fatal("ParseScript() error = nil, want syntax error")
			}
			if errors.Is(err, ErrIncompleteScript) {
				t.Fatalf("ParseScript() error = %v, want non-incomplete syntax error", err)
			}
		})
	}
}
