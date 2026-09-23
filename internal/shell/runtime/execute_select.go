package runtime

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
)

// `select name in words; do ... done` -- the last of the 46 constructs the v1.1 probe
// measured, and the only one it found missing.
//
// It is a `for` loop whose iteration is a question. The list is printed once as a
// numbered menu, then each pass reads a line: a number in range sets the name to that
// word, anything else sets it to the empty string, and an empty line reprints the menu
// without running the body. It ends on end-of-input, or on `break`.
//
// Four details, each of which is the one people notice when it is wrong:
//
//   - **The menu and the prompt go to stderr**, not stdout. That is what lets
//     `choice=$(select ...)` capture the answer rather than the menu.
//   - **REPLY is the line as typed**, not the chosen word, and it is set even when the
//     answer was not a valid number. A script that validates its own input needs the raw
//     text.
//   - **An out-of-range number is not an error.** The name becomes empty and the body
//     runs anyway, which is what makes the idiomatic `case $name in ... *) echo invalid`
//     work at all.
//   - **An empty line reprints the menu** and does not run the body, which is how a user
//     asks to see the list again.

// selectPrompt is PS3, whose default bash prints when nothing has set it.
const selectPrompt = "#? "

func (r Runtime) executeSelect(ctx context.Context, node loopNode, savedStatus int) lineResult {
	r.loops.enter()
	defer r.loops.leave()

	items, ok := r.selectItems(ctx, node, savedStatus)
	if !ok {
		return shellErrorResult()
	}
	// An empty list runs nothing at all, as an empty `for` does -- there is no question
	// to ask. bash agrees: `select x in; do echo no; done` prints nothing.
	if len(items) == 0 {
		return lineResult{status: 0}
	}
	input, err := r.fds.reader(0)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "select: %v\n", err)
		return lineResult{status: 1}
	}
	reader := bufio.NewReader(input)

	status := 0
	showMenu := true
	for {
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		if showMenu {
			r.writeSelectMenu(items)
		}
		showMenu = false
		fmt.Fprint(r.streams.Stderr, r.selectPromptText())
		line, readErr := reader.ReadString('\n')
		if line == "" && readErr != nil {
			// End of input ends the loop, which is what Ctrl-D does interactively.
			return lineResult{status: status}
		}
		line = strings.TrimRight(line, "\r\n")
		if r.assignVar("REPLY", line) != 0 {
			return r.loopVariableRefused()
		}
		if strings.TrimSpace(line) == "" {
			// A blank answer asks to see the list again, and does not run the body.
			showMenu = true
			if readErr != nil {
				return lineResult{status: status}
			}
			continue
		}
		if r.assignVar(node.name, selectChoice(items, line)) != 0 {
			return r.loopVariableRefused()
		}

		bodyStatus, control := r.executeProgram(ctx, node.body, savedStatus)
		status, savedStatus = bodyStatus, bodyStatus
		if ctx.Err() != nil {
			return lineResult{status: contextStatus(ctx)}
		}
		switch control {
		case flowNone:
		case flowContinue:
			// `continue 2` belongs to the loop outside this one, so a level that is
			// not ours is passed up unchanged -- the same rule every loop here follows.
			if !r.loops.consume() {
				return lineResult{status: 0, control: flowContinue}
			}
			status, savedStatus = 0, 0
		case flowBreak:
			if !r.loops.consume() {
				return lineResult{status: 0, control: flowBreak}
			}
			return lineResult{status: 0}
		default:
			return lineResult{status: status, control: control}
		}
		if readErr != nil {
			return lineResult{status: status}
		}
	}
}

// selectItems expands the word list, or the positional parameters when there is none.
func (r Runtime) selectItems(ctx context.Context, node loopNode, savedStatus int) ([]string, bool) {
	if node.overArguments {
		// `select name` with no list is `select name in "$@"`, the same default `for`
		// has and for the same reason.
		return append([]string(nil), r.params.values...), true
	}
	items := make([]string, 0, len(node.values))
	for _, item := range node.values {
		values := r.expandCommandWord(ctx, item, savedStatus)
		if r.shellErrorRaised() {
			return nil, false
		}
		items = append(items, values...)
	}
	return items, true
}

// writeSelectMenu prints the numbered list on stderr.
//
// In columns down then across, as bash does, with the numbers right-aligned so a list of
// more than nine items still lines up. One column here rather than bash's terminal-width
// packing: the width is not known for a non-terminal stream, and a menu that reflows
// depending on where it is piped is worse than one that is always a list.
func (r Runtime) writeSelectMenu(items []string) {
	width := len(strconv.Itoa(len(items)))
	for index, item := range items {
		fmt.Fprintf(r.streams.Stderr, "%*d) %s\n", width, index+1, item)
	}
}

// selectPromptText is PS3, or bash's default when it is unset.
func (r Runtime) selectPromptText() string {
	if prompt, ok := r.vars["PS3"]; ok {
		return prompt
	}
	return selectPrompt
}

// selectChoice maps an answer to a word, or to the empty string.
//
// Out of range is deliberately not an error: the empty name is what the conventional
// `case $name in ... *) echo "invalid";; esac` inside the body tests for.
func selectChoice(items []string, line string) string {
	number, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || number < 1 || number > len(items) {
		return ""
	}
	return items[number-1]
}
