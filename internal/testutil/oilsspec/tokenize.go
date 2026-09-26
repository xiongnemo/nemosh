package oilsspec

import (
	"fmt"
	"strings"
)

// lexMode is whether blank lines count, as in sh_spec.py's modal tokenizer.
type lexMode int

const (
	lexOuter lexMode = iota // between cases and ## lines, where blank lines are skipped
	lexRaw                  // inside code or a multi-line value, where they are kept
)

type tokenKind int

const (
	tokenCaseBegin tokenKind = iota // #### DESC
	tokenKeyValue                   // ## [QUALIFIER SHELLS] KEY: VALUE
	tokenMultiline                  // ## [QUALIFIER SHELLS] STDOUT: or STDERR:
	tokenEnd                        // ## END
	tokenPlain                      // a line of code or of expected output
	tokenEOF
)

type token struct {
	kind tokenKind
	line int
	// text is a case's description, or a plain line with its newline.
	text                           string
	qualifier, shells, name, value string
}

// tokenizer hands out one token at a time, each line classified in the mode the parser
// asks for.
type tokenizer struct {
	lines   []string
	read    int
	current token
}

// asciiSpace is what Python 2's str.strip removes, which is all sh_spec.py ever strips.
const asciiSpace = " \t\n\r\v\f"

func (t *tokenizer) next(mode lexMode) error {
	for {
		line := ""
		if t.read < len(t.lines) {
			line = t.lines[t.read]
		}
		t.read++
		item, skip, err := classify(line, t.read, mode)
		if err != nil {
			return err
		}
		if !skip {
			t.current = item
			return nil
		}
	}
}

// classify answers what one line is, or that it is to be skipped: a blank line outside
// code and output, or a comment anywhere. The order of the tests is sh_spec.py's.
func classify(line string, number int, mode lexMode) (token, bool, error) {
	switch {
	case line == "":
		return token{kind: tokenEOF, line: number}, false, nil
	case mode == lexOuter && strings.Trim(line, asciiSpace) == "":
		return token{}, true, nil
	case strings.HasPrefix(line, "####"):
		return token{kind: tokenCaseBegin, line: number, text: strings.Trim(line[4:], asciiSpace)}, false, nil
	}
	if match := keyValueLine.FindStringSubmatch(line); match != nil {
		item := token{kind: tokenKeyValue, line: number, qualifier: match[1], shells: match[2], name: match[3], value: match[4]}
		switch item.name {
		case "stdout", "stderr":
			// A one-line value stands for the line printed, newline and all.
			item.value += "\n"
		case "STDOUT", "STDERR":
			item.kind = tokenMultiline
		}
		return item, false, nil
	}
	switch {
	case endLine.MatchString(line):
		return token{kind: tokenEnd, line: number}, false, nil
	case strings.HasPrefix(line, "##"):
		return token{}, false, fmt.Errorf("line %d: invalid ## line %q", number, line)
	case strings.HasPrefix(strings.TrimLeft(line, asciiSpace), "#"):
		return token{}, true, nil
	}
	return token{kind: tokenPlain, line: number, text: line}, false, nil
}
