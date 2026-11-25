package main

import "strings"

// isCommandWrapper reports a command whose first operand is a command name: it runs it, or says
// what it is. timeout and su are not among them, though both run commands, because the word
// after their options is a duration and a user. A switch rather than a map, which would be
// built before main runs; see the init ceiling in product.yml.
func isCommandWrapper(name string) bool {
	switch name {
	case "builtin", "cdrop", "command", "drop", "env", "exec", "nohup", "pdrop", "sudo", "time", "type", "watch", "which", "xargs":
		return true
	}
	return false
}

// completesCommandName reports whether the word being typed is a command name: at the start of
// a command, as completesCommand decides, or after assignments, or after a wrapper and its
// options. `sudo gi` completes `git`, and so do `env FOO=1 gi` and `xargs -n 1 gi`.
//
// A wrapper's options are stepped over with the values they take, which the capability table
// knows for the wrappers this shell ships, sudo among them.
func (e *lineEditor) completesCommandName(prefix string) bool {
	if completesCommand(prefix) {
		return true
	}
	words := strings.Fields(commandSegment(prefix))
	index := 0
	for index < len(words) && isAssignmentWord(words[index]) {
		index++
	}
	for index < len(words) {
		wrapper := words[index]
		if !isCommandWrapper(wrapper) {
			return false
		}
		surface, _ := surfaceFor(e.specs, words[index:index+1])
		index++
		for index < len(words) {
			word := words[index]
			if wrapper == "env" && isAssignmentWord(word) {
				index++
				continue
			}
			if !strings.HasPrefix(word, "-") {
				break
			}
			index++
			if _, takesValue := detachedValueOption(surface, word); takesValue {
				index++
			}
		}
	}
	// Every word so far was an assignment, a wrapper or a wrapper's option, so the word being
	// typed is the command they run.
	return true
}

// isAssignmentWord reports whether a word is NAME=value.
func isAssignmentWord(word string) bool {
	name, _, found := strings.Cut(word, "=")
	return found && name != "" && isNamePrefix(name)
}
