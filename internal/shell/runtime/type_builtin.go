package runtime

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// typeBuiltin implements POSIX `type`: for each name, say how the shell would
// interpret it. It answers from the same order dispatch uses, so it cannot
// disagree with what actually runs, and it reports every name given rather than
// stopping at the first failure.
//
// With bash's options, which are how a script asks the question rather than a
// person: `-t` answers one word -- alias, keyword, function, builtin, file -- `-p`
// the path of a file and nothing otherwise, `-P` the path whatever else the name
// is, and `-a` every interpretation rather than the first. `-t` was refused, so
// `[ "$(type -t f)" = function ]` failed, and a keyword was `not found`. `-f` leaves
// functions out, as command does, so `type -f f` asks what f would be without its
// function; it was an invalid option.
func (r Runtime) typeBuiltin(args []string) int {
	mode, noFunctions, names, err := parseTypeOptions(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%stype: %v\n", r.diagnosticPrefix(), err)
		return 2
	}
	status := 0
	for _, name := range names {
		if !r.describeFor(mode, noFunctions, name) {
			status = 1
		}
	}
	return status
}

func parseTypeOptions(args []string) (byte, bool, []string, error) {
	mode, noFunctions := byte(0), false
	for index, argument := range args {
		if argument == "--" {
			return mode, noFunctions, args[index+1:], nil
		}
		if len(argument) < 2 || argument[0] != '-' {
			return mode, noFunctions, args[index:], nil
		}
		for _, letter := range argument[1:] {
			switch {
			case letter == 'f':
				noFunctions = true
			case strings.ContainsRune("tpPa", letter):
				mode = byte(letter)
			default:
				// bash's words: these are bash's options, which busybox's type has none of.
				return 0, false, nil, fmt.Errorf("-%c: invalid option", letter)
			}
		}
	}
	return mode, noFunctions, nil, nil
}

// describeFor answers one name in the given mode and reports whether it was found.
func (r Runtime) describeFor(mode byte, noFunctions bool, name string) bool {
	if mode == 'P' {
		resolved, err := r.externalCommandPath(name)
		if err != nil {
			return false
		}
		fmt.Fprintln(r.streams.Stdout, filepath.ToSlash(resolved))
		return true
	}
	kinds := r.commandKinds(name)
	if noFunctions {
		kinds = slices.DeleteFunc(kinds, func(kind commandKind) bool { return kind.word == "function" })
	}
	if len(kinds) == 0 {
		if mode != 't' && mode != 'p' {
			fmt.Fprintf(r.streams.Stderr, "%stype: %s: not found\n", r.diagnosticPrefix(), name)
		}
		return false
	}
	switch mode {
	case 't':
		fmt.Fprintln(r.streams.Stdout, kinds[0].word)
	case 'p':
		if kinds[0].word == "file" {
			fmt.Fprintln(r.streams.Stdout, kinds[0].path)
		}
	case 'a':
		for _, kind := range kinds {
			fmt.Fprintln(r.streams.Stdout, kind.description)
		}
	default:
		fmt.Fprintln(r.streams.Stdout, kinds[0].description)
	}
	return true
}

// ashBuiltinApplets are the applets busybox's shell has as builtins of its own, and so calls
// shell builtins, as bash does: `type echo` is "echo is a shell builtin" there. Here they are
// applets, and were "a builtin applet".
var ashBuiltinApplets = map[string]bool{"echo": true, "printf": true, "test": true, "[": true, "true": true, "false": true}

// appletBuiltins are the other way about: builtins here, which need the shell to run a command
// or search its PATH, and applets in busybox, which calls them builtin applets: `type which` is
// "which is a builtin applet" there. They were "a shell builtin".
var appletBuiltins = map[string]bool{"time": true, "timeout": true, "which": true}

// commandKind is one way the shell could read a name: the word `type -t` answers, the
// sentence `type` does, and the path when it is a file.
type commandKind struct {
	word        string
	description string
	path        string
}

// commandKinds is every interpretation of a name, in the order dispatch tries them: an
// alias, a reserved word, a special builtin, a function, a builtin, an applet, a file on
// PATH. A function comes before an ordinary builtin, as runCommandResolved has it; this
// used to put every builtin first, so after `cd() { ...; }` it said `cd` was a builtin
// while running `cd` ran the function.
func (r Runtime) commandKinds(name string) []commandKind {
	var kinds []commandKind
	if value, ok := r.aliases[name]; ok {
		kinds = append(kinds, commandKind{word: "alias", description: fmt.Sprintf("%s is an alias for %s", name, value)})
	}
	if ReservedWord(name) {
		kinds = append(kinds, commandKind{word: "keyword", description: name + " is a shell keyword"})
	}
	// A special builtin is called one, as busybox calls POSIX's and its own local and times:
	// "eval is a special shell builtin". It was "a shell builtin", bash's words for both.
	if isRuntimeBuiltin(name) && isSpecialBuiltin(name) {
		kinds = append(kinds, commandKind{word: "builtin", description: name + " is a special shell builtin"})
	}
	// The function dispatch would call, which a name with a slash never is: it is a path, by
	// busybox's rule and POSIX's. `f/g() { ...; }` was called a function, and running f/g
	// looked for the file.
	if _, found := r.calledFunction(name); found {
		kinds = append(kinds, commandKind{word: "function", description: name + " is a function"})
	}
	if isRuntimeBuiltin(name) && !isSpecialBuiltin(name) {
		description := name + " is a shell builtin"
		switch {
		case errorEndsShell(name):
			description = name + " is a special shell builtin"
		case appletBuiltins[name]:
			description = name + " is a builtin applet"
		}
		kinds = append(kinds, commandKind{word: "builtin", description: description})
	}
	if builtin, ok := unimplementedBuiltins[name]; ok {
		// "and will not" separates a gap from a decision, which is the thing
		// `type` is being asked about.
		description := name + " is a shell builtin this shell does not implement"
		if builtin.permanent {
			description += " and will not"
		}
		kinds = append(kinds, commandKind{word: "builtin", description: description})
	}
	if _, ok := r.lookupApplet(name); ok {
		// busybox's own words for the same thing, and it is the primary reference
		// (AGENTS.md). An applet runs inside the shell, so to `-t` it is a builtin:
		// a script asking for "file" is asking whether it would start a program.
		description := name + " is a builtin applet"
		if ashBuiltinApplets[name] {
			description = name + " is a shell builtin"
		}
		kinds = append(kinds, commandKind{word: "builtin", description: description})
	}
	if resolved, err := r.externalCommandPath(name); err == nil {
		path := filepath.ToSlash(resolved)
		kinds = append(kinds, commandKind{word: "file", description: fmt.Sprintf("%s is %s", name, path), path: path})
	}
	return kinds
}
