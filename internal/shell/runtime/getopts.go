package runtime

import (
	"fmt"
	"strconv"
	"strings"
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
// flag leaves OPTARG empty rather than unset, and the diagnostics are busybox's words.
func (r Runtime) getopts(args []string) int {
	if len(args) < 2 {
		fmt.Fprintln(r.streams.Stderr, "getopts: expected optstring and name")
		return 2
	}
	optstring, name := args[0], args[1]
	words := r.params.values
	if len(args) > 2 {
		words = args[2:]
	}
	silent := strings.HasPrefix(optstring, ":")
	spec := strings.TrimPrefix(optstring, ":")
	word, sub := r.getoptsIndex(), 0
	// Inside a group, `-ab`: the letters after the first are read from where the last call
	// stopped, as long as OPTIND is still what that call left.
	if state := r.params.getopts; state.sub > 0 && state.reported == word {
		word, sub = state.word, state.sub
	}
	r.params.getopts = getoptsState{}
	if sub == 0 {
		if word > len(words) {
			return r.endGetopts(name, word)
		}
		arg := words[word-1]
		if arg == "--" {
			return r.endGetopts(name, word+1)
		}
		if len(arg) < 2 || arg[0] != '-' {
			return r.endGetopts(name, word)
		}
		sub = 1
	}
	arg := words[word-1]
	letter := arg[sub]
	sub++
	next := word + 1
	position := strings.IndexByte(spec, letter)
	switch {
	case position < 0 || letter == ':':
		r.getoptsProblem(name, silent, "?", letter, fmt.Sprintf("Illegal option -%c", letter))
	case position+1 < len(spec) && spec[position+1] == ':':
		switch {
		case sub < len(arg):
			r.getoptsFound(name, letter, arg[sub:])
		case word < len(words):
			r.getoptsFound(name, letter, words[word])
			next = word + 2
		default:
			r.getoptsProblem(name, silent, ":", letter, fmt.Sprintf("No arg for -%c option", letter))
		}
		sub = len(arg)
	default:
		r.getoptsFound(name, letter, "")
	}
	if sub < len(arg) {
		r.params.getopts = getoptsState{word: word, sub: sub, reported: next}
	}
	_ = r.assignVar("OPTIND", strconv.Itoa(next))
	return 0
}

// getoptsState is where a group of letters was left: the word, the next letter in it, and
// the OPTIND the last call reported, which is how a later call knows it is continuing.
type getoptsState struct {
	word, sub, reported int
}

func (r Runtime) getoptsFound(name string, letter byte, argument string) {
	_ = r.assignVar("OPTARG", argument)
	_ = r.assignVar(name, string(letter))
}

// getoptsProblem is an unknown option or a missing argument. Silent mode, a leading `:`,
// reports it through the name -- `?` or `:` -- with the letter in OPTARG; otherwise it is
// said on stderr and the name is `?`.
func (r Runtime) getoptsProblem(name string, silent bool, silentName string, letter byte, message string) {
	if silent {
		_ = r.assignVar("OPTARG", string(letter))
		_ = r.assignVar(name, silentName)
		return
	}
	fmt.Fprintln(r.streams.Stderr, message)
	delete(r.vars, "OPTARG")
	r.markVarMutation("OPTARG")
	_ = r.assignVar(name, "?")
}

func (r Runtime) getoptsIndex() int {
	index, err := strconv.Atoi(r.vars["OPTIND"])
	if err != nil || index < 1 {
		return 1
	}
	return index
}

// endGetopts is the end of the options: status 1, the name `?`, and OPTIND at the first
// operand -- past a `--`, which both references step over.
func (r Runtime) endGetopts(name string, optind int) int {
	_ = r.assignVar(name, "?")
	_ = r.assignVar("OPTIND", strconv.Itoa(optind))
	return 1
}
