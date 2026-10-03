package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// programmableCompletion asks the shell what its `complete` specifications answer for the line
// and the cursor's byte offset in it, and whether one applied. The session sets it; an editor
// with no shell behind it has none, and completes as it always has.
type programmableCompletion func(line string, point int) (runtime.ProgrammableCompletion, bool)

// completeProgrammatically puts in what a specification answered, as readline puts in what
// bash's programmable completion answers: the candidates replace the text from the answer's
// start to the cursor; one is put in whole -- quoted only when -o filenames says they are names
// -- and followed by a blank unless -o nospace, or by a slash when it names a directory; several
// put in what they share, and are listed, in their own order under -o nosort. It reports false
// when the editor should complete as it would without a specification: nothing was answered, and
// -o default or -o bashdefault asks for the ordinary completion then.
func (e *lineEditor) completeProgrammatically(answer runtime.ProgrammableCompletion, prompt string) bool {
	line := e.buffer.String()
	point := len(string(e.buffer.runes[:e.buffer.cursor]))
	typed := line[min(answer.Start, point):point]
	options := answer.Options
	matches := answer.Candidates
	if len(matches) == 0 {
		if options["default"] || options["bashdefault"] {
			return false
		}
		fmt.Fprint(e.screen, "\a")
		return true
	}
	insert := func(text string) string {
		if options["filenames"] && !options["noquote"] {
			return escapeForInsertion(text)
		}
		return text
	}
	if !options["nosort"] {
		sortCandidates(matches)
		matches = slicesCompact(matches)
	}
	if len(matches) == 1 {
		e.replaceWord(typed, insert(matches[0]))
		switch {
		case options["filenames"] && strings.HasSuffix(matches[0], "/"):
		case options["filenames"] && e.namesDirectory(matches[0]):
			e.buffer.insert('/')
		case !options["nospace"]:
			e.buffer.insert(' ')
		}
		return true
	}
	if shared := longestSharedPrefix(matches); len(shared) > len(typed) && strings.HasPrefix(shared, typed) {
		e.replaceWord(typed, insert(shared))
	}
	e.listCandidatesAsGiven(matches, prompt)
	return true
}

// namesDirectory reports whether a completion names a directory, read from the editor's working
// directory as the shell would read it.
func (e *lineEditor) namesDirectory(name string) bool {
	if !filepath.IsAbs(name) {
		name = filepath.Join(e.workingDirectory, name)
	}
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}
