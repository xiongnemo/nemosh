package runtime

import (
	"fmt"
	"strings"
)

// The condition of an `if`, a `while` or an `until` is a compound_list (POSIX 2.9.4): any
// number of commands, on any number of lines, compounds among them, up to the `then` or the
// `do`. Its status is the last one's.
//
// **It was the header line and nothing else.** A span's condition was parsed from the text
// after the keyword, so a second line of condition was silently dropped -- `if true`,
// `false`, `then` took the yes branch, and a `while` whose counter was stepped on the
// condition's second line never stepped it and never ended. And a compound as the
// condition -- `if case $x in ...; esac; then`, `while case ...` -- was a syntax error,
// because its arms and its closer arrived as lines the span builder had no place for. A
// bare `if` on its own line, which is how some scripts start a long condition, was not an
// opener at all.
//
// Now the condition is every line from the header to the `then` or `do`, parsed as the
// program it is -- nested spans included -- and handed to the executor as one brace group,
// which is how a compound used where a command is expected is represented everywhere else.
// A group's status is its last command's, which is exactly a compound_list's.

// conditionKeywords open a compound whose header is a condition rather than a name or a word.
var conditionKeywords = [...]string{"if", "while", "until"}

// splitCompoundConditions puts a compound that begins a condition on a line of its own:
// `if case a in` becomes `if` and `case a in`, so that the case-arm pass and the span
// builder, which find a compound at the start of a line, find this one too. Repeated, so
// `if if true` becomes `if` and `if true`.
func splitCompoundConditions(lines []string) []string {
	var split []string
	for _, line := range lines {
		for {
			keyword, header, ok := compoundConditionHeader(line)
			if !ok {
				split = append(split, line)
				break
			}
			split = append(split, keyword)
			line = header
		}
	}
	return split
}

// compoundConditionHeader reports a line whose condition begins with a compound keyword.
func compoundConditionHeader(line string) (string, string, bool) {
	for _, keyword := range conditionKeywords {
		header, ok := compoundHeader(line, keyword)
		if !ok {
			continue
		}
		header = strings.TrimLeft(header, " \t")
		if beginsWithCompoundKeyword(header) {
			return keyword, header, true
		}
	}
	return "", "", false
}

// bareConditionOpener is a condition keyword alone on its line: the condition follows on
// the lines after it.
func bareConditionOpener(line string) (compoundKind, bool) {
	switch line {
	case "if":
		return compoundIf, true
	case "while", "until":
		return compoundLoop, true
	}
	return 0, false
}

// parseCondition reads a condition: the header, then every line up to end, the index of the
// `then` or `do`. The ordinary one-line condition is parsed exactly as it always was.
func parseCondition(lines []string, spans []compoundSpan, byStart map[int]int, span compoundSpan, keyword string, end int, budget *parseBudget, depth int) (list, error) {
	header, _ := compoundHeader(spanHeaderLine(lines, span), keyword)
	if span.start+1 >= end {
		return parseTypedLineWithBudget(header, budget, depth)
	}
	var program []programNode
	if strings.TrimSpace(header) != "" {
		first, err := parseTypedLineWithBudget(header, budget, depth)
		if err != nil {
			return list{}, err
		}
		if len(first.items) > 0 {
			program = append(program, listNode{value: first})
		}
	}
	rest, err := parseTypedProgram(lines, spans, byStart, span.start+1, end, budget, depth)
	if err != nil {
		return list{}, err
	}
	program = append(program, rest...)
	if len(program) == 0 {
		return list{}, fmt.Errorf("syntax error: %s with no condition", keyword)
	}
	group := braceGroup{body: Script{program: program}}
	return list{items: []listItem{{value: andOr{pipelines: []pipeline{{commands: []commandNode{group}}}}}}}, nil
}
