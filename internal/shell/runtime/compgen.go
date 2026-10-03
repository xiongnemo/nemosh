package runtime

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// compgen is bash's: the completions of a word, one to a line, as its actions, a glob, a word
// list, a function and a command would generate them, filtered by -X and given -P and -S. It
// answers 1 when there are none. busybox has no programmable completion, so bash is the
// reference; nemosh's own completion for the line editor is its TOML specs, which this does not
// read.
//
//	compgen [-V varname] [-abcdefgjksuv] [-o option] [-A action] [-G globpat] [-W wordlist]
//	        [-F function] [-C command] [-X filterpat] [-P prefix] [-S suffix] [word]
//
// The results come in bash's order whatever the options' order: the actions the shell knows
// (aliases, builtins, functions, keywords, variables and the rest), then the ones on the
// filesystem (commands, directories, files, users), then -G, -W, -F and -C.
func (r Runtime) compgen(ctx context.Context, args []string, savedStatus int) int {
	spec, word, ok := r.parseCompgen(args)
	if !ok {
		return 2
	}
	matches := r.compgenActions(spec.actions, word)
	if spec.glob != "" {
		matches = append(matches, r.compgenGlob(ctx, spec.glob, savedStatus)...)
	}
	if spec.hasWords {
		words, ok := r.compgenWords(ctx, spec.words, savedStatus)
		if !ok {
			return 1
		}
		matches = append(matches, withPrefix(words, word)...)
	}
	if spec.function != "" {
		replies, ok := r.compgenFunction(ctx, spec.function, word, savedStatus)
		if !ok {
			return 1
		}
		matches = append(matches, replies...)
	}
	if spec.command != "" {
		matches = append(matches, r.compgenCommand(ctx, spec.command, word, savedStatus)...)
	}
	if spec.filter != "" {
		matches = r.compgenFilter(matches, spec.filter, word)
	}
	for index, match := range matches {
		matches[index] = spec.prefix + match + spec.suffix
	}
	switch {
	case spec.options["plusdirs"]:
		matches = append(matches, r.compgenPaths(word, true)...)
	case len(matches) == 0 && spec.options["dirnames"]:
		matches = r.compgenPaths(word, true)
	case len(matches) == 0 && (spec.options["default"] || spec.options["bashdefault"]):
		matches = r.compgenPaths(word, false)
	}
	if spec.array != "" {
		r.arrays.set(spec.array, matches)
		delete(r.vars, spec.array)
	} else {
		for _, match := range matches {
			fmt.Fprintln(r.streams.Stdout, match)
		}
	}
	if len(matches) == 0 {
		return 1
	}
	return 0
}

type compgenSpec struct {
	actions                                []string
	glob, words, function, command, filter string
	prefix, suffix, array                  string
	hasWords                               bool
	options                                map[string]bool
}

var compgenLetters = map[byte]string{
	'a': "alias", 'b': "builtin", 'c': "command", 'd': "directory", 'e': "export", 'f': "file",
	'g': "group", 'j': "job", 'k': "keyword", 's': "service", 'u': "user", 'v': "variable",
}

var compgenOptionNames = []string{"bashdefault", "default", "dirnames", "filenames", "noquote", "nosort", "nospace", "plusdirs", "fullquote"}

// parseCompgen reads compgen's options, as bash's internal_getopt does: letters clustered, an
// option's value in the rest of its word or the next, and the word to complete the first operand.
func (r Runtime) parseCompgen(args []string) (compgenSpec, string, bool) {
	spec := compgenSpec{options: map[string]bool{}}
	index := 0
	for ; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			break
		}
		for at := 1; at < len(arg); at++ {
			letter := arg[at]
			if action, ok := compgenLetters[letter]; ok {
				spec.actions = append(spec.actions, action)
				continue
			}
			if !strings.ContainsRune("oAGWFCXPSV", rune(letter)) {
				r.compgenUsage(fmt.Sprintf("-%c: invalid option", letter))
				return spec, "", false
			}
			value := arg[at+1:]
			if value == "" {
				if index+1 == len(args) {
					r.compgenUsage(fmt.Sprintf("-%c: option requires an argument", letter))
					return spec, "", false
				}
				index++
				value = args[index]
			}
			if !spec.set(r, letter, value) {
				return spec, "", false
			}
			break
		}
	}
	word := ""
	if index < len(args) {
		word = args[index]
	}
	return spec, word, true
}

func (spec *compgenSpec) set(r Runtime, letter byte, value string) bool {
	switch letter {
	case 'o':
		if !slices.Contains(compgenOptionNames, value) {
			fmt.Fprintf(r.streams.Stderr, "%scompgen: %s: invalid option name\n", r.diagnosticPrefix(), value)
			return false
		}
		spec.options[value] = true
	case 'A':
		if !slices.Contains(compgenActionOrder, value) {
			fmt.Fprintf(r.streams.Stderr, "%scompgen: %s: invalid action name\n", r.diagnosticPrefix(), value)
			return false
		}
		spec.actions = append(spec.actions, value)
	case 'G':
		spec.glob = value
	case 'W':
		spec.words, spec.hasWords = value, true
	case 'F':
		spec.function = value
	case 'C':
		spec.command = value
	case 'X':
		spec.filter = value
	case 'P':
		spec.prefix = value
	case 'S':
		spec.suffix = value
	case 'V':
		spec.array = value
	}
	return true
}

func (r Runtime) compgenUsage(message string) {
	fmt.Fprintf(r.streams.Stderr, "%scompgen: %s\n", r.diagnosticPrefix(), message)
	fmt.Fprintln(r.streams.Stderr, "compgen: usage: compgen [-V varname] [-abcdefgjksuv] [-o option] [-A action] [-G globpat] [-W wordlist] [-F function] [-C command] [-X filterpat] [-P prefix] [-S suffix] [word]")
}

// withPrefix keeps the candidates that begin with word, in their order.
func withPrefix(candidates []string, word string) []string {
	var kept []string
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, word) {
			kept = append(kept, candidate)
		}
	}
	return kept
}
