package runtime

import (
	"fmt"
	"strings"
)

// lineNumbering places what is being parsed in the source it came from, which is what
// $LINENO reports: the line a command starts on.
//
// The parser does not work on the source's own lines. Heredoc bodies are taken out
// first, continued lines are joined, one line can become several (`a; b`, `then echo x`,
// a one-line case), and a group's body or a command substitution is parsed again as a
// text of its own. So each pass hands its lines on with where they started, and every
// command is stamped with its line as it is built. Everything is counted in the lines
// left once the heredoc bodies are out, and origins turns that count back into the
// source's.
//
// A command continued onto a new line inside a list -- `a &&` then `b` on the next --
// reports the line its list began on, because the join leaves no trace of where the
// second line started. Both references say the line b is on.
type lineNumbering struct {
	// first is the source line the text begins on: 1 for a script, and the running
	// command's line for text parsed while it runs -- eval, a trap, a session's input.
	first int
	// origins is, for each line left once the heredoc bodies were taken out, the index of
	// the source line it was. Nil when there were no bodies to take out.
	origins []int
	// base is the index, among those lines, of the first line of the text being parsed:
	// 0 for a script, and where a nested text sits for a group's body.
	base int
	// at is, for each line the passes hand on, the index its text starts at.
	at []int
	// current is the index of the line whose commands are being built.
	current int
}

// number is the source line for an index among the lines left once heredoc bodies were
// taken out.
func (n lineNumbering) number(index int) int {
	if index >= 0 && index < len(n.origins) {
		index = n.origins[index]
	}
	return max(n.first, 1) + index
}

// line is the source line of the commands being built now.
func (budget *parseBudget) line() int {
	return budget.numbering.number(budget.numbering.current)
}

// enterLine makes the index'th line handed to the parser the one being built.
func (budget *parseBudget) enterLine(index int) {
	if index >= 0 && index < len(budget.numbering.at) {
		budget.numbering.current = budget.numbering.at[index]
	}
}

// lineOf is the source line of the index'th line handed to the parser.
func (budget *parseBudget) lineOf(index int) int {
	if index >= 0 && index < len(budget.numbering.at) {
		return budget.numbering.number(budget.numbering.at[index])
	}
	return budget.line()
}

// numberLines records where each line of the text being parsed starts, from the offsets
// logicalLines found for them.
func (budget *parseBudget) numberLines(starts []int) {
	at := make([]int, len(starts))
	for index, start := range starts {
		at[index] = budget.numbering.base + start
	}
	budget.numbering.at = at
}

// parseNestedScript parses text found inside the line being built, after before -- a
// group's body, a command substitution -- as a script of its own, numbered from where it
// sits in that line. The numbering is the enclosing parse's again afterwards.
func parseNestedScript(text, before string, budget *parseBudget, depth int) (Script, error) {
	saved := budget.numbering
	defer func() { budget.numbering = saved }()
	budget.numbering.base = saved.current + strings.Count(before, "\n")
	return parseScript(text, budget, depth+1)
}

// parseScriptAt parses text that begins on source line first. ParseScript is this at 1.
func parseScriptAt(source string, first int) (Script, error) {
	if len(source) > maxParseInputBytes {
		return Script{}, fmt.Errorf("input bytes: %w", errParseLimit)
	}
	return parseScript(source, &parseBudget{numbering: lineNumbering{first: first}}, 0)
}

// appendNumbered adds a line to a pass's output with where it starts, which is the line
// it was cut from.
func appendNumbered(lines []string, at []int, line string, start int) ([]string, []int) {
	return append(lines, line), append(at, start)
}

// startOf is where the index'th line starts, for a pass handed lines with no record of
// where they came from.
func startOf(at []int, index int) int {
	if index < len(at) {
		return at[index]
	}
	return 0
}

// enterLine makes line the running command's. Zero is a command with no line -- one
// built rather than parsed -- which leaves the last one standing.
func (r Runtime) enterLine(line int) {
	if line > 0 && r.expansion != nil {
		r.expansion.line = line
	}
}

// currentLine is $LINENO: the running command's line, and 1 before any has run.
func (r Runtime) currentLine() int {
	if r.expansion == nil || r.expansion.line == 0 {
		return 1
	}
	return r.expansion.line
}

// parseHere parses text the running command hands over -- eval's words, a trap's action,
// a substitution met while expanding -- numbered from the running command's line, which is
// what both references report inside them.
func (r Runtime) parseHere(text string) (Script, error) {
	return parseScriptAt(text, r.currentLine())
}
