package runtime

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseRedirects_supportsNumberedFormsAndSeparatedTargets(t *testing.T) {
	tokens, err := scanShellTokens("probe 3>&1 4<&- 5>out 6< input")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	command, redirects, err := parseRedirects(tokens)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if got, want := tokenValues(command), []string{"probe"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("command: got %v want %v", got, want)
	}
	want := []redirectOperation{
		{kind: redirectDup, target: 3, source: 1},
		{kind: redirectClose, target: 4},
		{kind: redirectOutput, target: 5, path: "out", operand: word{parts: []wordPart{{kind: wordPartLiteral, text: "out"}}}},
		{kind: redirectInput, target: 6, path: "input", operand: word{parts: []wordPart{{kind: wordPartLiteral, text: "input"}}}},
	}
	if !reflect.DeepEqual(redirects, want) {
		t.Fatalf("redirects:\n got %#v\nwant %#v", redirects, want)
	}
}

func TestFinalSecurity_parseRedirects_supportsAppend(t *testing.T) {
	// Given
	tokens, err := scanShellTokens("echo value 3>>output")
	if err != nil {
		t.Fatalf("scan append redirect: %v", err)
	}

	// When
	_, redirects, err := parseRedirects(tokens)

	// Then
	if err != nil {
		t.Fatalf("parse append redirect: %v", err)
	}
	want := []redirectOperation{{kind: redirectAppend, target: 3, path: "output", operand: word{parts: []wordPart{{kind: wordPartLiteral, text: "output"}}}}}
	if !reflect.DeepEqual(redirects, want) {
		t.Fatalf("append redirects: got %#v want %#v", redirects, want)
	}
}

func TestParseRedirects_rejectsUnsupportedMalformedAndOutOfRangeForms(t *testing.T) {
	tests := []struct {
		input string
		want  error
	}{
		{"echo >", errMissingRedirectTarget},
		{"echo 256>out", errInvalidDescriptor},
		{"echo 2>&+1", errMalformedRedirect},
		{"echo 2>&x", errMalformedRedirect},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			tokens, err := scanShellTokens(test.input)
			if err != nil {
				t.Fatalf("scan: %v", err)
			}
			_, _, err = parseRedirects(tokens)
			if !errors.Is(err, test.want) {
				t.Fatalf("parse error: got %v want %v", err, test.want)
			}
		})
	}
}
