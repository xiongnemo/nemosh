package runtime

import (
	"fmt"
	"strconv"
)

// getopts is POSIX getopts, with busybox-w32's answers where it and bash part.
//
// It read one option per word and nothing else: `-ab` ended the parse because the word was
// not a single letter, an unknown option ended it too instead of reporting `?` for the
// `case` to handle, there was no silent mode, no attached argument (`-bval`), and the
// operands after the name -- `getopts ab: opt "$@"`, the form a function uses -- were
// ignored in favour of the positional parameters. Each of those is ordinary, and a script
// that met one stopped reading its options without saying why.
//
// Where the references differ busybox's answer is kept: OPTIND already points past a word
// while its letters are still being read (bash holds it back until the word is done), a
// flag leaves OPTARG empty rather than unset, OPTERR=0 is silent mode as a leading `:` is,
// and the diagnostics are busybox's words.
//
// Where it is in the words is its own, as busybox keeps it, and OPTIND reports it; see
// getoptsState. It was read back from OPTIND, so a parse after `set --` or `shift` carried
// on from where the last one ended, and an OPTIND past the words stayed there.
func (r Runtime) getopts(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(r.streams.Stderr, "getopts: usage: getopts optstring var [arg]")
		return 2
	}
	optstring, name := args[0], args[1]
	words := r.params.values
	if len(args) > 2 {
		words = args[2:]
	}
	// Past the end of the words it starts again from the first, as busybox's getoptscmd does.
	place := r.params.getopts
	if place.word() > len(words)+1 {
		place = getoptsState{}
	}
	next, sub := place.word(), place.sub
	arg := ""
	if next > 1 && sub > 0 && sub < len(words[next-2]) {
		arg = words[next-2]
	} else {
		if next > len(words) || len(words[next-1]) < 2 || words[next-1][0] != '-' {
			return r.endGetopts(name, next)
		}
		arg, sub = words[next-1], 1
		next++
		if arg == "--" {
			return r.endGetopts(name, next)
		}
	}
	letter := arg[sub]
	sub++
	silent := optstring != "" && optstring[0] == ':' || r.vars["OPTERR"] == "0"
	value := string(letter)
	known, takesArgument := getoptsLetter(optstring, letter)
	switch {
	case !known:
		value = r.getoptsProblem(silent, "?", letter, fmt.Sprintf("Illegal option -%c", letter))
	case takesArgument:
		switch {
		case sub < len(arg):
			_ = r.assignVar("OPTARG", arg[sub:])
		case next <= len(words):
			_ = r.assignVar("OPTARG", words[next-1])
			next++
		default:
			value = r.getoptsProblem(silent, ":", letter, fmt.Sprintf("No arg for -%c option", letter))
		}
		sub = len(arg)
	default:
		_ = r.assignVar("OPTARG", "")
	}
	if sub >= len(arg) {
		sub = 0
	}
	return r.reportGetopts(name, value, getoptsState{next: next, sub: sub}, 0)
}

// getoptsState is where getopts is in its words, busybox's shellparam.optind and optoff: the
// word it reads next, from 1, and where the next letter is in the word before it, 0 once that
// word is done. The zero value is a parse at its start.
//
// It is kept with the positional parameters, so a function's parse is its own and the
// caller's is where it was when the call returns; `set --` and `shift` start it again, and so
// does assigning or unsetting OPTIND, at the word the value names (markVarMutation). That is
// the only time OPTIND is read, as in busybox.
type getoptsState struct {
	next, sub int
}

// word is the word getopts reads next: 1 for a parse at its start.
func (s getoptsState) word() int {
	return max(s.next, 1)
}

// getoptsFrom is the place an OPTIND assignment leaves, busybox's getoptsreset: the word the
// value names, and the start for anything but digits naming a word.
func getoptsFrom(optind string) getoptsState {
	if !isDigits(optind) {
		return getoptsState{}
	}
	next, _ := strconv.Atoi(optind)
	return getoptsState{next: next}
}

// getoptsLetter finds a letter as busybox's getopts scans the optstring: a `:` after a letter
// says it takes an argument. The scan starts at the first character, so after a leading `:`
// a `-:` is a flag of its own, as it is there.
func getoptsLetter(optstring string, letter byte) (known, takesArgument bool) {
	for index := 0; index < len(optstring); index++ {
		if optstring[index] == letter {
			return true, index+1 < len(optstring) && optstring[index+1] == ':'
		}
		if index+1 < len(optstring) && optstring[index+1] == ':' {
			index++
		}
	}
	return false, false
}

// getoptsProblem is an unknown option or a missing argument, and answers with what the name
// is given. Silent mode reports it through the name -- `?` or `:` -- with the letter in
// OPTARG; otherwise it is said on stderr, OPTARG is unset, and the name is `?`.
func (r Runtime) getoptsProblem(silent bool, silentName string, letter byte, message string) string {
	if silent {
		_ = r.assignVar("OPTARG", string(letter))
		return silentName
	}
	fmt.Fprintln(r.streams.Stderr, message)
	r.unsetName("OPTARG")
	return "?"
}

// endGetopts is the end of the options: status 1, OPTARG unset, the name `?`, and OPTIND at
// the first operand -- past a `--`, which both references step over.
func (r Runtime) endGetopts(name string, next int) int {
	r.unsetName("OPTARG")
	return r.reportGetopts(name, "?", getoptsState{next: next}, 1)
}

// reportGetopts sets OPTIND and then the name, in busybox's order: a name that cannot be
// assigned -- not a variable's, or readonly -- is status 2 with OPTARG and OPTIND already
// set, and the next call starts from the first word again, as busybox's does. It was
// assigned anyway, and the call answered 0.
func (r Runtime) reportGetopts(name, value string, place getoptsState, status int) int {
	_ = r.assignVar("OPTIND", strconv.Itoa(place.next))
	switch {
	case !isValidVariableName(name):
		fmt.Fprintf(r.streams.Stderr, "getopts: %s: bad variable name\n", name)
		place, status = getoptsState{}, 2
	case r.isReadonly(name):
		fmt.Fprintf(r.streams.Stderr, "getopts: %s: readonly variable\n", name)
		place, status = getoptsState{}, 2
	default:
		_ = r.assignVar(name, value)
	}
	r.params.getopts = place
	return status
}
