package runtime

import (
	"fmt"
	"path/filepath"
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
// `[ "$(type -t f)" = function ]` failed, and a keyword was `not found`.
func (r Runtime) typeBuiltin(args []string) int {
	mode, names, err := parseTypeOptions(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "type: %v\n", err)
		return 2
	}
	status := 0
	for _, name := range names {
		if !r.describeFor(mode, name) {
			status = 1
		}
	}
	return status
}

func parseTypeOptions(args []string) (byte, []string, error) {
	mode := byte(0)
	for index, argument := range args {
		if argument == "--" {
			return mode, args[index+1:], nil
		}
		if len(argument) < 2 || argument[0] != '-' {
			return mode, args[index:], nil
		}
		for _, letter := range argument[1:] {
			if !strings.ContainsRune("tpPa", letter) {
				return 0, nil, fmt.Errorf("-%c: not an option this build has; it takes -t -p -P -a", letter)
			}
			mode = byte(letter)
		}
	}
	return mode, nil, nil
}

// describeFor answers one name in the given mode and reports whether it was found.
func (r Runtime) describeFor(mode byte, name string) bool {
	if mode == 'P' {
		resolved, err := r.externalCommandPath(name)
		if err != nil {
			return false
		}
		fmt.Fprintln(r.streams.Stdout, filepath.ToSlash(resolved))
		return true
	}
	kinds := r.commandKinds(name)
	if len(kinds) == 0 {
		if mode != 't' && mode != 'p' {
			fmt.Fprintf(r.streams.Stderr, "type: %s: not found\n", name)
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
	if isRuntimeBuiltin(name) && isSpecialBuiltin(name) {
		kinds = append(kinds, commandKind{word: "builtin", description: name + " is a shell builtin"})
	}
	if parsed, ok := newFunctionName(name); ok {
		if _, found := r.functions[parsed]; found {
			kinds = append(kinds, commandKind{word: "function", description: name + " is a function"})
		}
	}
	if isRuntimeBuiltin(name) && !isSpecialBuiltin(name) {
		kinds = append(kinds, commandKind{word: "builtin", description: name + " is a shell builtin"})
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
		kinds = append(kinds, commandKind{word: "builtin", description: name + " is a builtin applet"})
	}
	if resolved, err := r.externalCommandPath(name); err == nil {
		path := filepath.ToSlash(resolved)
		kinds = append(kinds, commandKind{word: "file", description: fmt.Sprintf("%s is %s", name, path), path: path})
	}
	return kinds
}
