package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// getopts in bash mode is bash's sh_getopt (lib/sh/getopt.c) and dogetopts, where busybox's
// getoptscmd parts from it. The parse's place is getoptsState read bash's way: next is the word
// being read, bash's sh_optind, and sub the next letter in it, 0 between words -- so OPTIND,
// which is next, names the word until its last letter has been read. A flag leaves OPTARG
// unset; `set --` and shift leave the place as it was, as they leave OPTIND; a place past the
// words ends the parse with OPTIND one past them; a name that is no variable's is status 1 with
// OPTIND and OPTARG already set; and the diagnostics are bash's, `$0: illegal option -- z`.
// Assigning OPTIND starts the parse again at the word it names, in both.

// getoptsAsBash is one call of getopts in bash mode.
func (r Runtime) getoptsAsBash(optstring, name string, words []string) int {
	next, sub := r.params.getopts.word(), r.params.getopts.sub
	if next > len(words) {
		r.unsetName("OPTARG")
		return r.reportGetoptsAsBash(name, "?", getoptsState{next: len(words) + 1}, 1)
	}
	word := words[next-1]
	if sub == 0 {
		switch {
		case word == "--":
			r.unsetName("OPTARG")
			return r.reportGetoptsAsBash(name, "?", getoptsState{next: next + 1}, 1)
		case len(word) < 2 || word[0] != '-':
			r.unsetName("OPTARG")
			return r.reportGetoptsAsBash(name, "?", getoptsState{next: next}, 1)
		}
		sub = 1
	}
	letter := word[sub]
	sub++
	if sub >= len(word) {
		next, sub = next+1, 0
	}
	silent := optstring != "" && optstring[0] == ':'
	quiet := silent || r.vars["OPTERR"] == "0"
	at := strings.IndexByte(optstring, letter)
	value := string(letter)
	switch {
	case at < 0 || letter == ':':
		value = r.getoptsProblemAsBash(silent, quiet, "?", letter, "illegal option")
	case at+1 < len(optstring) && optstring[at+1] == ':':
		switch {
		case sub != 0:
			_ = r.assignVar("OPTARG", word[sub:])
			next, sub = next+1, 0
		case next > len(words):
			value = r.getoptsProblemAsBash(silent, quiet, ":", letter, "option requires an argument")
		default:
			_ = r.assignVar("OPTARG", words[next-1])
			next++
		}
	default:
		r.unsetName("OPTARG")
	}
	return r.reportGetoptsAsBash(name, value, getoptsState{next: next, sub: sub}, 0)
}

// getoptsProblemAsBash is an option getopts has not got or an argument missing, as bash's
// dogetopts answers it: silent, through the name, `?` or `:`, with the letter in OPTARG;
// otherwise `?`, OPTARG unset, and bash's words on stderr after $0 unless OPTERR is 0.
func (r Runtime) getoptsProblemAsBash(silent, quiet bool, silentName string, letter byte, what string) string {
	if silent {
		_ = r.assignVar("OPTARG", string(letter))
		return silentName
	}
	if !quiet {
		fmt.Fprintf(r.streams.Stderr, "%s: %s -- %c\n", r.params.name, what, letter)
	}
	r.unsetName("OPTARG")
	return "?"
}

// reportGetoptsAsBash sets OPTIND and then the name, as bash's dogetopts does: a name that is no
// variable's is said, status 1, the place kept.
func (r Runtime) reportGetoptsAsBash(name, value string, place getoptsState, status int) int {
	_ = r.assignVar("OPTIND", strconv.Itoa(place.next))
	r.params.getopts = place
	if !isValidVariableName(name) {
		return r.notIdentifier("getopts", name)
	}
	if refused := r.assignVar(name, value); refused != 0 {
		return refused
	}
	return status
}
