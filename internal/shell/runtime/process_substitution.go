package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// `<(command)` -- process substitution.
//
// `diff <(sort a) <(sort b)` is the shape it exists for: two commands compared without two
// temporary files written by hand. It was a syntax error, because `<` is a redirect
// operator and `(command)` a subshell, so the halves were read as different things.
//
// On Windows this needs deciding rather than translating. bash substitutes `/dev/fd/63`, a
// path to the reading end of a pipe, and Windows has no `/dev/fd`: a child cannot be handed
// a descriptor by naming it. The two ways out are a named pipe -- `\\.\pipe\name`, which is
// openable by path -- and a temporary file.
//
// This uses a temporary file, and the difference is observable, so it is worth stating. The
// substituted command runs to completion *before* the consumer starts reading, where bash
// runs them concurrently. `<(yes)` would fill a disk here and stream for ever there, and a
// consumer that stops early does not stop the producer. What it buys is that the path
// behaves like any other file: readable twice, seekable, and usable by an external program
// that knows nothing about this shell -- none of which a Windows named pipe can promise,
// being one-shot and forward-only. `[[ -f <(echo x) ]]` is therefore true here and false in
// bash, where the path names a pipe.
//
// That is the right trade for what the form is used for: comparing or reading the output of
// a command that ends. `>(command)` is refused by name rather than approximated, because
// writing into a file a command has already finished reading is not the same thing.

// processSubstitutionOpensAt reports whether the `(` at index is the one in `<(` or `>(`.
//
// `>(` too, although it is refused: the refusal has to come from the lexer naming the form,
// which it cannot do if an earlier scan has already torn the line apart at the `)`.
func processSubstitutionOpensAt(line string, index int) bool {
	if index == 0 || index >= len(line) || line[index] != '(' {
		return false
	}
	if index >= 2 && line[index-2] == '\\' {
		// The operator was escaped, so it is data.
		return false
	}
	return line[index-1] == '<' || line[index-1] == '>'
}

// wordGroupOpensAt reports whether the `(` at index belongs to the word in front of it
// rather than opening a subshell.
//
// Both forms that do this -- an extended pattern group and a process substitution -- have to
// be stepped over whole by every scan that has an opinion about parentheses, and the reason
// is the same for both: the parenthesis is part of one word, so a `;`, `|` or `)` inside it
// is not the outer grammar's. Eight scans, which is what the first attempt at each of these
// got wrong by changing four of them. See pattern_extended.go for the count.
func wordGroupOpensAt(line string, index int) bool {
	return extendedGroupOpensAt(line, index) || processSubstitutionOpensAt(line, index)
}

// processSubstitutionPart reads the `<(command)` at index into a word part, answering an end
// of zero when there is none there.
//
// Called from the lexer before the operator check, which would otherwise take the `<` for a
// redirect and leave `(command)` as a subshell. Here rather than there because lexer.go is
// at its line ceiling, and because the form's own file is where a reader would look for it.
func processSubstitutionPart(line string, index int, budget *parseBudget, depth int) (wordPart, int, error) {
	if line[index] != '<' && line[index] != '>' {
		return wordPart{}, 0, nil
	}
	if index+1 >= len(line) || line[index+1] != '(' {
		return wordPart{}, 0, nil
	}
	end, ok := matchingParenthesis(line, index+1)
	if !ok {
		return wordPart{}, 0, nil
	}
	text := line[index : end+1]
	// Refused before the body is parsed, so the diagnostic is about the form rather than
	// about whatever is inside it.
	if line[index] == '>' {
		return wordPart{}, 0, fmt.Errorf(
			"process substitution %s: only the input form <(...) is implemented", text)
	}
	nested, err := parseScript(line[index+2:end], budget, depth+1)
	if err != nil {
		return wordPart{}, 0, err
	}
	return wordPart{
		kind:   wordPartProcessSubstitution,
		text:   text,
		quote:  quoteUnquoted,
		script: &nested,
	}, end, nil
}

// expandProcessSubstitution runs the script and answers with a path to its output.
//
// The file is registered for removal rather than deleted here: the consumer has not opened
// it yet, and a path that is gone by then would be worse than no feature.
func (r Runtime) expandProcessSubstitution(ctx context.Context, script *Script, savedStatus int) string {
	if script == nil {
		return ""
	}
	file, err := os.CreateTemp("", "nemosh-procsub-*")
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: process substitution: %v\n", err)
		return ""
	}
	r.expansion.registerProcessSubstitution(file.Name())
	if err := r.runIntoFile(ctx, *script, file, savedStatus); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: process substitution: %v\n", err)
		return ""
	}
	// Forward slashes, which is the spelling every operand in this shell is resolved
	// through: the consumer opens it the same way it would open a path the user typed.
	return filepath.ToSlash(file.Name())
}

// runIntoFile runs the script in a child whose stdout is the file.
//
// The same shape commandSubstitutionScript uses, and for the same reasons: a snapshot so
// the substituted command cannot see the consumer's descriptors, no inherited traps, and
// the job scope drained before the file is closed so nothing is still writing to it.
func (r Runtime) runIntoFile(ctx context.Context, script Script, file *os.File, savedStatus int) error {
	child, err := r.snapshot(ctx)
	if err != nil {
		return err
	}
	table := child.fds
	if err := table.bindBorrowedWriter(1, file); err != nil {
		child.jobScope.cancelAndDrain()
		return errors.Join(err, table.closeAll())
	}
	child = child.withFDTable(table)
	child.traps = map[trapName]string{}
	// The status is discarded, which is bash's behaviour: `diff <(false) x` reports what
	// diff said, not what false said.
	child.executeTypedScriptFrom(ctx, script, savedStatus)
	child.jobScope.cancelAndDrain()
	return errors.Join(table.closeAll(), file.Close())
}

// cleanUpProcessSubstitutions removes the files a command's substitutions left behind.
//
// After the command rather than after the expansion, because the consumer reads the file in
// between. Called from the one place that knows a command has finished, so a substitution
// inside a loop does not leave one file per iteration.
func (r Runtime) cleanUpProcessSubstitutions() {
	for _, path := range r.expansion.takeProcessSubstitutions() {
		// A failure to remove is not reported: the file is in the system temporary
		// directory, the command has already run, and a diagnostic arriving after the
		// output it belongs to explains nothing the reader can act on.
		_ = os.Remove(path)
	}
}
