package runtime

import (
	"errors"
	"strings"
	"testing"
)

func TestParseScript_buildsTypedGroupCommands_withTrailingRedirectsAndPipeline(t *testing.T) {
	script, err := ParseScript("{ echo grouped; } >out | (cat)\n")
	if err != nil {
		t.Fatal(err)
	}
	commands := script.program[0].(listNode).value.items[0].value.pipelines[0].commands
	if len(commands) != 2 {
		t.Fatalf("commands = %d, want 2", len(commands))
	}
	group, ok := commands[0].(braceGroup)
	if !ok || len(group.redirects) != 1 {
		t.Fatalf("command 0 = %#v, want redirected braceGroup", commands[0])
	}
	if _, ok := commands[1].(subshellCommand); !ok {
		t.Fatalf("command 1 = %T, want subshellCommand", commands[1])
	}
}

func TestParseScript_doesNotResolveUserWordAsGeneratedGroupPlaceholder(t *testing.T) {
	// Given / When
	script, err := ParseScript("__nemosh_group_0__ && (echo first) && (echo second)\n")

	// Then
	if err != nil {
		t.Fatal(err)
	}
	pipelines := script.program[0].(listNode).value.items[0].value.pipelines
	if _, ok := pipelines[0].commands[0].(simpleCommand); !ok {
		t.Fatalf("first command = %T, want simpleCommand", pipelines[0].commands[0])
	}
	if _, ok := pipelines[1].commands[0].(subshellCommand); !ok {
		t.Fatalf("second command = %T, want subshellCommand", pipelines[1].commands[0])
	}
	if _, ok := pipelines[2].commands[0].(subshellCommand); !ok {
		t.Fatalf("third command = %T, want subshellCommand", pipelines[2].commands[0])
	}
}

func TestScanExtractedGroups_carriesOpaqueGroupIdentityOnGeneratedTokenOnly(t *testing.T) {
	// Given
	masked, groups, err := extractGroupCommands("__nemosh_group_0__ && (echo grouped)", &parseBudget{}, 0)
	if err != nil {
		t.Fatal(err)
	}

	// When
	tokens, err := scanExtractedGroups(masked, groups, &parseBudget{}, 0)

	// Then
	if err != nil {
		t.Fatal(err)
	}
	if tokens[0].group != nil {
		t.Fatalf("literal token group = %#v, want nil", tokens[0].group)
	}
	if tokens[2].group == nil || tokens[2].group.brace {
		t.Fatalf("generated token group = %#v, want subshell identity", tokens[2].group)
	}
}

func TestParseScript_classifiesGroupClosers(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		incomplete bool
	}{
		{name: "missing brace", source: "{ echo value;", incomplete: true},
		{name: "missing parenthesis", source: "(echo value", incomplete: true},
		// `echo value }` is not here because it is not an error at all: the `}`
		// follows a word, so bash and dash both print `value }` and exit 0, and
		// so does Nemosh. A `}` needs command position to be the closer.
		{name: "unexpected brace", source: "echo value; }", incomplete: false},
		{name: "unexpected parenthesis", source: "echo value )", incomplete: false},
		// The `}` here is text, so the group is still open at end of input --
		// which is what bash ("unexpected end of file") and dash ("end of file
		// unexpected (expecting \"}\")") both report.
		{name: "brace lacks separator", source: "{ echo value }", incomplete: true},
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

func TestParseScript_treatsQuotedAndEscapedGroupCharactersAsWords(t *testing.T) {
	script, err := ParseScript(`echo '{' \( "}" \)`)
	if err != nil {
		t.Fatal(err)
	}
	command := firstSimpleCommand(t, script)
	if len(command.words) != 5 {
		t.Fatalf("words = %d, want 5", len(command.words))
	}
}

func TestParseScript_enforcesSharedDepthAcrossGroups(t *testing.T) {
	// Spaced, because `((` is now an arithmetic command rather than two groups, and
	// an unspaced run of parentheses is no longer group nesting at all. It is still
	// safe -- 5,000 of them parse and run without crashing, because the scan that
	// steps over `((expr))` is iterative -- but it is not what this test is about.
	source := strings.Repeat("( ", maxParseDepth+1) + "true" + strings.Repeat(" )", maxParseDepth+1)
	_, err := ParseScript(source)
	if !errors.Is(err, errParseLimit) {
		t.Fatalf("ParseScript() error = %v, want errParseLimit", err)
	}
}

func TestParseScript_parsesMultilineGroupContainingTypedCompound(t *testing.T) {
	script, err := ParseScript("{\nif true\nthen\necho nested\nfi\n}\n")
	if err != nil {
		t.Fatal(err)
	}
	group := script.program[0].(listNode).value.items[0].value.pipelines[0].commands[0].(braceGroup)
	if _, ok := group.body.program[0].(ifNode); !ok {
		t.Fatalf("body node = %T, want ifNode", group.body.program[0])
	}
}

func TestParseScript_enforcesBraceReservedWordBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		incomplete bool
	}{
		{name: "opening brace joined to command", source: "{echo bad;}", incomplete: false},
		{name: "closing brace joined to suffix", source: "{ echo ok; }suffix", incomplete: true},
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

func TestParseScript_acceptsBraceReservedWordsAndMixedNesting(t *testing.T) {
	for _, source := range []string{
		"{ echo ok; }",
		"{ (echo nested); }",
		"( { echo nested; } )",
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := ParseScript(source); err != nil {
				t.Fatalf("ParseScript() error = %v", err)
			}
		})
	}
}

func TestParseScript_rejectsCrossedGroupDelimitersAsMalformed(t *testing.T) {
	_, err := ParseScript("( { echo mixed; ) echo leaked; }")
	if err == nil || errors.Is(err, ErrIncompleteScript) {
		t.Fatalf("ParseScript() error = %v, want complete syntax error", err)
	}
}
