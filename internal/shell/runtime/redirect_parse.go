package runtime

import (
	"errors"
	"fmt"
	"strconv"
)

var (
	errUnsupportedRedirect   = errors.New("unsupported redirection")
	errMissingRedirectTarget = errors.New("missing redirection target")
	errAmbiguousRedirect     = errors.New("ambiguous redirection target")
	errMalformedRedirect     = errors.New("malformed redirection")
)

type redirectKind uint8

const (
	redirectInput redirectKind = iota
	redirectOutput
	// redirectClobber is `>|`: truncate even under `set -C`, which is the only
	// thing that tells it apart from `>`.
	redirectClobber
	// redirectReadWrite is `<>`: open for both without truncating, and create
	// the file if it is not there.
	redirectReadWrite
	redirectAppend
	redirectHeredoc
	// redirectHereString is `<<<word`: the word, expanded, with one newline after
	// it. A heredoc whose body is written on the same line, which is what makes
	// `read v <<< "$x"` the usual way to feed one string to something.
	redirectHereString
	redirectDup
	redirectClose
)

type redirectOperation struct {
	kind          redirectKind
	target        int
	source        int
	path          string
	operand       word
	delimiterWord word
	delimiter     string
	expand        bool
	stripTabs     bool
	// bothStreams is `&>`: stderr follows stdout to the same place. Kept as a flag
	// rather than expanded into two operations so the pair cannot be separated by
	// ordering, which is the bug `>file 2>&1` written backwards already is.
	bothStreams bool
	body        string
	line        int
	order       int
}

func parseRedirects(tokens []shellToken) ([]shellToken, []redirectOperation, error) {
	return parseRedirectsWithBudget(tokens, nil)
}

func parseRedirectsWithBudget(tokens []shellToken, budget *parseBudget) ([]shellToken, []redirectOperation, error) {
	command := make([]shellToken, 0, len(tokens))
	operations := make([]redirectOperation, 0)
	for index := 0; index < len(tokens); index++ {
		token := tokens[index]
		if token.kind != tokenRedirect {
			command = append(command, token)
			continue
		}
		operation, needsOperand, err := parseRedirectToken(token.value)
		if err != nil {
			return nil, nil, err
		}
		if needsOperand {
			index++
			if index >= len(tokens) || tokens[index].kind != tokenWord {
				return nil, nil, fmt.Errorf("%s: %w", token.value, errMissingRedirectTarget)
			}
			operand := tokens[index].value
			switch operation.kind {
			case redirectHeredoc:
				if budget == nil {
					return nil, nil, fmt.Errorf("%s: %w", token.value, errUnsupportedRedirect)
				}
				record, ok := budget.heredoc(operand)
				if !ok {
					return nil, nil, fmt.Errorf("%s: %w", token.value, errMissingRedirectTarget)
				}
				operation.delimiterWord = record.delimiterWord
				operation.delimiter = record.delimiter
				operation.expand = record.expand
				operation.stripTabs = record.stripTabs
				operation.body = record.body
				operation.line = record.line
				operation.order = record.order
			case redirectHereString:
				// The operand is the body rather than a path, so it is kept as a
				// word and expanded with the rest of the command. Not read here:
				// `<<< "$x"` has to see the value $x holds when the command runs.
				operation.operand = parseTypedWord(*tokens[index].parsed)
			case redirectInput, redirectOutput, redirectClobber, redirectReadWrite, redirectAppend:
				operation.path = operand
				operation.operand = parseTypedWord(*tokens[index].parsed)
			case redirectDup, redirectClose:
				operation, _, err = parseDupRedirect(operation.target, operand, token.value+operand)
				if err != nil {
					return nil, nil, err
				}
			}
		}
		operations = append(operations, operation)
	}
	if len(command) == 0 {
		return nil, nil, errors.New("empty command after redirection")
	}
	return command, operations, nil
}

func parseRedirectToken(value string) (redirectOperation, bool, error) {
	digits := 0
	for digits < len(value) && value[digits] >= '0' && value[digits] <= '9' {
		digits++
	}
	if digits == len(value) {
		return redirectOperation{}, false, fmt.Errorf("%q: %w", value, errMalformedRedirect)
	}
	operator := value[digits:]
	defaultFD := 1
	if operator[0] == '<' {
		defaultFD = 0
	}
	target, err := parseDescriptor(value[:digits], defaultFD)
	if err != nil {
		return redirectOperation{}, false, err
	}
	if operator == "<&" || operator == ">&" {
		return redirectOperation{kind: redirectDup, target: target}, true, nil
	}
	if operator == "<" {
		return redirectOperation{kind: redirectInput, target: target}, true, nil
	}
	if operator == "<<<" {
		return redirectOperation{kind: redirectHereString, target: target}, true, nil
	}
	if operator == "<<" || operator == "<<-" {
		return redirectOperation{kind: redirectHeredoc, target: target, stripTabs: operator == "<<-"}, true, nil
	}
	if operator == "&>" {
		return redirectOperation{kind: redirectOutput, target: 1, bothStreams: true}, true, nil
	}
	if operator == "&>>" {
		return redirectOperation{kind: redirectAppend, target: 1, bothStreams: true}, true, nil
	}
	if operator == ">" {
		return redirectOperation{kind: redirectOutput, target: target}, true, nil
	}
	if operator == ">>" {
		return redirectOperation{kind: redirectAppend, target: target}, true, nil
	}
	if operator == ">|" {
		return redirectOperation{kind: redirectClobber, target: target}, true, nil
	}
	if operator == "<>" {
		return redirectOperation{kind: redirectReadWrite, target: target}, true, nil
	}
	return redirectOperation{}, false, fmt.Errorf("%q: %w", value, errMalformedRedirect)
}

func parseDupRedirect(target int, sourceText, value string) (redirectOperation, bool, error) {
	if sourceText == "-" {
		return redirectOperation{kind: redirectClose, target: target}, false, nil
	}
	if sourceText == "" {
		return redirectOperation{}, false, fmt.Errorf("%q: %w", value, errMissingRedirectTarget)
	}
	source, err := parseDescriptor(sourceText, -1)
	if err != nil {
		return redirectOperation{}, false, fmt.Errorf("%q: %w", value, errMalformedRedirect)
	}
	return redirectOperation{kind: redirectDup, target: target, source: source}, false, nil
}

func parseDescriptor(text string, defaultValue int) (int, error) {
	if text == "" {
		return defaultValue, nil
	}
	if !isDigits(text) {
		return 0, fmt.Errorf("descriptor %q: %w", text, errInvalidDescriptor)
	}
	value, err := strconv.Atoi(text)
	if err != nil || value < 0 || value > maxDescriptor {
		return 0, fmt.Errorf("descriptor %q: %w", text, errInvalidDescriptor)
	}
	return value, nil
}

// takesPath reports whether a redirection's operand is a pathname that has to
// be expanded, as opposed to a descriptor number or a heredoc body.
func (k redirectKind) takesPath() bool {
	switch k {
	case redirectInput, redirectOutput, redirectClobber, redirectReadWrite, redirectAppend:
		return true
	}
	return false
}
