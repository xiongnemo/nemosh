package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// completionRequest is the line a completion is for, as a specification's function and command
// are told it: the command, the word, the word before it, COMP_WORDS and COMP_CWORD, COMP_LINE,
// and COMP_POINT in characters. compgen's has no line: COMP_CWORD -1 and the rest empty.
type completionRequest struct {
	command, word, previous string
	words                   []string
	cword                   int
	line                    string
	point                   int
	// interactive marks the line editor's, which COMP_TYPE and COMP_KEY say is a Tab's.
	interactive bool
}

// generateCompletions is bash's gen_compspec_completions: the actions, -G, -W, -F and -C, in that
// order whatever the order they were given in, then -X, -P and -S, and the directories -o
// dirnames and -o plusdirs add. It answers false when -W or the function failed, and retry when
// the function answered 124, which asks for the specification to be looked up again.
func (r Runtime) generateCompletions(ctx context.Context, spec compgenSpec, request completionRequest, savedStatus int) (matches []string, retry, ok bool) {
	word := request.word
	matches = r.compgenActions(spec.actions, word)
	if spec.glob != "" {
		matches = append(matches, r.compgenGlob(ctx, spec.glob, savedStatus)...)
	}
	if spec.hasWords {
		words, ok := r.compgenWords(ctx, spec.words, savedStatus)
		if !ok {
			return nil, false, false
		}
		matches = append(matches, withPrefix(words, word)...)
	}
	if spec.function != "" {
		replies, status, ok := r.completionFunction(ctx, spec.function, request, savedStatus)
		if !ok {
			return nil, false, false
		}
		if status == 124 {
			return nil, true, true
		}
		matches = append(matches, replies...)
	}
	if spec.command != "" {
		matches = append(matches, r.completionCommand(ctx, spec.command, request, savedStatus)...)
	}
	if spec.filter != "" {
		matches = r.compgenFilter(matches, spec.filter, word)
	}
	for index, match := range matches {
		matches[index] = spec.prefix + match + spec.suffix
	}
	switch {
	case len(matches) == 0 && spec.options["dirnames"]:
		matches = r.compgenPaths(word, true)
	case spec.options["plusdirs"]:
		matches = append(matches, r.compgenPaths(word, true)...)
	}
	return matches, false, true
}

// completionFunction is -F: the function called as bash calls it, with the command, the word and
// the word before it, and COMP_WORDS, COMP_CWORD, COMP_LINE and COMP_POINT saying the line, and
// COMP_TYPE and COMP_KEY a Tab when the line editor asked; COMPREPLY is what it answers, and all
// of them are unset again after, as bash unsets them. It answers the function's status, and
// false when there is no such function or it failed with a shell error.
func (r Runtime) completionFunction(ctx context.Context, name string, request completionRequest, savedStatus int) ([]string, int, bool) {
	definition, found := r.calledFunction(name)
	if !found {
		caller := "completion"
		if !request.interactive {
			caller = "compgen"
		}
		fmt.Fprintf(r.streams.Stderr, "%s%s: function `%s' not found\n", r.diagnosticPrefix(), caller, name)
		return nil, 0, false
	}
	variables := []string{"COMP_WORDS", "COMP_CWORD", "COMP_LINE", "COMP_POINT", "COMP_TYPE", "COMP_KEY", "COMPREPLY"}
	defer func() {
		for _, variable := range variables {
			r.arrays.unset(variable)
			delete(r.vars, variable)
		}
	}()
	r.arrays.set("COMP_WORDS", request.words)
	r.vars["COMP_CWORD"], r.vars["COMP_LINE"] = strconv.Itoa(request.cword), request.line
	r.vars["COMP_POINT"] = strconv.Itoa(request.point)
	if request.interactive {
		r.vars["COMP_TYPE"], r.vars["COMP_KEY"] = "9", "9"
	}
	r.arrays.unset("COMPREPLY")
	delete(r.vars, "COMPREPLY")
	result := r.callFunctionResult(ctx, definition, []string{request.command, request.word, request.previous}, savedStatus)
	if r.expansion.shellError || result.control == flowAbort {
		r.discardLine()
		return nil, result.status, false
	}
	if r.arrays.has("COMPREPLY") {
		var replies []string
		for _, index := range r.arrays.liveIndices("COMPREPLY") {
			value, _ := r.arrays.valueAt("COMPREPLY", index)
			replies = append(replies, value)
		}
		return replies, result.status, true
	}
	if value, set := r.vars["COMPREPLY"]; set {
		return []string{value}, result.status, true
	}
	return nil, result.status, true
}

// completionCommand is -C: the command run, as bash runs it, with the command, the word and the
// word before it as its operands and COMP_LINE and COMP_POINT -- and COMP_TYPE and COMP_KEY,
// from the line editor -- in its environment; each line of its output is a completion.
func (r Runtime) completionCommand(ctx context.Context, command string, request completionRequest, savedStatus int) []string {
	environment := "COMP_LINE=" + shellquote.Single(request.line) + " COMP_POINT=" + strconv.Itoa(request.point)
	if request.interactive {
		environment += " COMP_TYPE=9 COMP_KEY=9"
	}
	operands := shellquote.Single(request.command) + " " + shellquote.Single(request.word) + " " + shellquote.Single(request.previous)
	script, err := r.parseHere(environment + " " + command + " " + operands)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%scompgen: %v\n", r.diagnosticPrefix(), err)
		return nil
	}
	output := r.commandSubstitutionScript(ctx, script, savedStatus)
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}
