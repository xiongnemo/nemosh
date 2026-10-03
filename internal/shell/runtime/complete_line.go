package runtime

import (
	"context"
	"maps"
	"strings"
	"unicode/utf8"
)

// The line editor's programmable completion: the specification complete made for the command
// the cursor is in, run as bash's programmable_completions runs it, with the line said in
// COMP_WORDS, COMP_CWORD, COMP_LINE and COMP_POINT, and its options handed back for the editor
// to put the answer in by.

// ProgrammableCompletion is what a specification answered for the word at the cursor.
type ProgrammableCompletion struct {
	// Candidates are the completions as the specification made them: COMPREPLY is not
	// filtered by the word, as bash does not filter it.
	Candidates []string
	// Start is where in the line the text they replace begins: after the last character of
	// COMP_WORDBREAKS before the cursor that no quote holds, as readline's word begins.
	Start int
	// Options are the specification's -o options once it has run, compopt's changes included.
	Options map[string]bool
}

// defaultCompletionWordBreaks is bash's COMP_WORDBREAKS, which a session starts with.
const defaultCompletionWordBreaks = " \t\n\"'@><=;|&(:"

// completionSeparators end one command and begin the next, as bash's COMMAND_SEPARATORS do,
// and a newline.
const completionSeparators = ";|&{(`\n"

// CompleteLine runs the specification for the command the cursor is in, point being a byte
// offset in line. It answers false when there is none to run -- the cursor is in a command's
// name, or nothing was said for the command and nothing for every command -- and the editor
// completes as it does without one.
func (r Runtime) CompleteLine(ctx context.Context, line string, point int) (ProgrammableCompletion, bool) {
	point = min(max(point, 0), len(line))
	start, end := completionCommandBounds(line, point)
	name, offset, ok := completionCommandName(line[start:point])
	if strings.TrimSpace(line) == "" {
		name, offset, ok = "", point-start, true
	}
	if !ok {
		return ProgrammableCompletion{}, false
	}
	// The command's line begins at its name, as bash's s1 does: the assignments before it, and
	// the blanks, are no part of COMP_LINE.
	start += offset
	breaks, set := r.vars["COMP_WORDBREAKS"]
	if !set {
		breaks = defaultCompletionWordBreaks
	}
	wordStart := completionWordStart(line[:point], breaks)
	words, cword := completionWords(line[start:end], point-start, breaks)
	request := completionRequest{
		command: name, word: line[wordStart:point], words: words, cword: cword,
		line: line[start:end], point: utf8.RuneCountInString(line[start:point]), interactive: true,
	}
	if cword > 0 {
		request.previous = words[cword-1]
	}
	// A function that answers 124 has loaded a specification, as `complete -D -F
	// _completion_loader` does, and the lookup starts again: at most 32 times, as in bash.
	for range 32 {
		key, found := r.completionSpecFor(name)
		if !found {
			return ProgrammableCompletion{}, false
		}
		spec := r.completions.specs[key].clone()
		previous := r.completions.running
		r.completions.running = &runningCompletion{name: key, spec: &spec}
		matches, retry, _ := r.generateCompletions(ctx, spec, request, 0)
		r.completions.running = previous
		if !retry {
			return ProgrammableCompletion{Candidates: matches, Start: wordStart, Options: maps.Clone(spec.options)}, true
		}
	}
	return ProgrammableCompletion{}, false
}

// completionSpecFor is the name a command's specification is kept under: its own, its last
// path component's, or -D's; and -E's for the empty line.
func (r Runtime) completionSpecFor(name string) (string, bool) {
	candidates := []string{emptyCompletion}
	if name != "" {
		candidates = []string{name}
		if slash := strings.LastIndexByte(name, '/'); slash >= 0 && slash < len(name)-1 {
			candidates = append(candidates, name[slash+1:])
		}
		candidates = append(candidates, defaultCompletion)
	}
	for _, key := range candidates {
		if _, found := r.completions.specs[key]; found {
			return key, true
		}
	}
	return "", false
}

// completionCommandBounds is where the command the cursor is in begins and ends: after the
// last separator before the cursor that no quote holds, and at the first one after it, as
// bash's find_cmd_start and find_cmd_end find them.
func completionCommandBounds(line string, point int) (int, int) {
	quoted, _ := completionQuoted(line)
	start := 0
	for index := range len(line) {
		if quoted[index] || strings.IndexByte(completionSeparators, line[index]) < 0 {
			continue
		}
		if line[index] == '{' && !completionBraceOpens(line, index) {
			continue
		}
		if index >= point {
			return start, index
		}
		start = index + 1
	}
	return start, len(line)
}

// completionBraceOpens reports a `{` that is the reserved word: where a command begins and
// followed by a blank, which `echo {a,b}`'s is not.
func completionBraceOpens(line string, index int) bool {
	before := strings.TrimRight(line[:index], " \t")
	after := index+1 < len(line) && (line[index+1] == ' ' || line[index+1] == '\t')
	return after && (before == "" || strings.IndexByte(completionSeparators, before[len(before)-1]) >= 0)
}

// completionCommandName is the name of the command whose text up to the cursor is given, past
// the assignments before it, and where in the text it begins. It answers false while the cursor
// is still in the name or in an assignment before it, where a command is being completed and
// not an operand.
func completionCommandName(text string) (string, int, bool) {
	for _, field := range completionFields(text) {
		if field.end == len(text) {
			return "", 0, false
		}
		if !isAssignment(field.text) {
			return unquoteCompletionWord(field.text), field.end - len(field.text), true
		}
	}
	return "", 0, false
}

// completionField is a word of a half-typed command and where it ends.
type completionField struct {
	text string
	end  int
}

// completionFields splits text at the blanks no quote holds.
func completionFields(text string) []completionField {
	quoted, _ := completionQuoted(text)
	var fields []completionField
	start := -1
	for index := 0; index <= len(text); index++ {
		blank := index == len(text) || !quoted[index] && (text[index] == ' ' || text[index] == '\t' || text[index] == '\n')
		switch {
		case blank && start >= 0:
			fields = append(fields, completionField{text: text[start:index], end: index})
			start = -1
		case !blank && start < 0:
			start = index
		}
	}
	return fields
}

// unquoteCompletionWord takes the quotes and backslashes off a command name, `"git"` and `g\it`
// being git.
func unquoteCompletionWord(text string) string {
	var out strings.Builder
	quote := byte(0)
	for index := 0; index < len(text); index++ {
		char := text[index]
		switch {
		case quote == 0 && (char == '\'' || char == '"'):
			quote = char
		case char == quote:
			quote = 0
		case char == '\\' && quote != '\'' && index+1 < len(text):
			index++
			out.WriteByte(text[index])
		default:
			out.WriteByte(char)
		}
	}
	return out.String()
}
