package runtime

import (
	"fmt"
	"slices"
	"strings"
)

// The options compgen and complete share, read as bash's build_actions reads them for both.

type compgenSpec struct {
	actions                                []string
	glob, words, function, command, filter string
	prefix, suffix, array                  string
	hasWords                               bool
	options                                map[string]bool
}

// clone is the specification with its own options, so a compopt on one command's leaves the
// others named in the same `complete` alone.
func (spec compgenSpec) clone() compgenSpec {
	spec.actions = slices.Clone(spec.actions)
	options := make(map[string]bool, len(spec.options))
	for name, on := range spec.options {
		options[name] = on
	}
	spec.options = options
	return spec
}

var compgenLetters = map[byte]string{
	'a': "alias", 'b': "builtin", 'c': "command", 'd': "directory", 'e': "export", 'f': "file",
	'g': "group", 'j': "job", 'k': "keyword", 's': "service", 'u': "user", 'v': "variable",
}

// compgenOptionNames are the -o names, in bash's compopts order, which is how they print.
var compgenOptionNames = []string{"bashdefault", "default", "dirnames", "filenames", "fullquote", "noquote", "nosort", "nospace", "plusdirs"}

// completionUsages are the usage lines bash prints after a misused option.
var completionUsages = map[string]string{
	"compgen":  "compgen: usage: compgen [-V varname] [-abcdefgjksuv] [-o option] [-A action] [-G globpat] [-W wordlist] [-F function] [-C command] [-X filterpat] [-P prefix] [-S suffix] [word]",
	"complete": "complete: usage: complete [-abcdefgjksuv] [-pr] [-DEI] [-o option] [-A action] [-G globpat] [-W wordlist] [-F function] [-C command] [-X filterpat] [-P prefix] [-S suffix] [name ...]",
	"compopt":  "compopt: usage: compopt [-o|+o option] [-DEI] [name ...]",
}

// completionOptions is what an option list said: the specification, the letters that stand
// alone -- complete's -p, -r, -D, -E and -I -- and whether any option was given at all, which
// decides what complete does with no names.
type completionOptions struct {
	spec  compgenSpec
	flags string
	given bool
}

func (options completionOptions) has(letter byte) bool {
	return strings.IndexByte(options.flags, letter) >= 0
}

// parseCompletionOptions reads a builtin's options as bash's internal_getopt does: letters
// clustered, an option's value in the rest of its word or the next. flags are the letters the
// builtin takes alone besides the actions, and values the ones that take a value. It answers
// the options and where the operands begin.
func (r Runtime) parseCompletionOptions(builtin string, args []string, flags, values string) (completionOptions, int, bool) {
	options := completionOptions{spec: compgenSpec{options: map[string]bool{}}}
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
		options.given = true
		for at := 1; at < len(arg); at++ {
			letter := arg[at]
			if action, ok := compgenLetters[letter]; ok {
				options.spec.actions = append(options.spec.actions, action)
				continue
			}
			if strings.IndexByte(flags, letter) >= 0 {
				options.flags += string(letter)
				continue
			}
			if strings.IndexByte(values, letter) < 0 {
				r.completionUsage(builtin, fmt.Sprintf("-%c: invalid option", letter))
				return options, index, false
			}
			value := arg[at+1:]
			if value == "" {
				if index+1 == len(args) {
					r.completionUsage(builtin, fmt.Sprintf("-%c: option requires an argument", letter))
					return options, index, false
				}
				index++
				value = args[index]
			}
			if !options.spec.set(r, builtin, letter, value) {
				return options, index, false
			}
			break
		}
	}
	return options, index, true
}

func (spec *compgenSpec) set(r Runtime, builtin string, letter byte, value string) bool {
	switch letter {
	case 'o':
		if !slices.Contains(compgenOptionNames, value) {
			fmt.Fprintf(r.streams.Stderr, "%s%s: %s: invalid option name\n", r.diagnosticPrefix(), builtin, value)
			return false
		}
		spec.options[value] = true
	case 'A':
		if !slices.Contains(compgenActionOrder, value) {
			fmt.Fprintf(r.streams.Stderr, "%s%s: %s: invalid action name\n", r.diagnosticPrefix(), builtin, value)
			return false
		}
		spec.actions = append(spec.actions, value)
	case 'G':
		spec.glob = value
	case 'W':
		spec.words, spec.hasWords = value, true
	case 'F':
		// A function's name, which the specification prints bare and calls by name: a blank
		// or an operator in it is no name, as bash's check_identifier has it.
		if builtin == "complete" && (value == "" || strings.ContainsAny(value, " \t\n()<>;&|")) {
			fmt.Fprintf(r.streams.Stderr, "%s%s: `%s': not a valid identifier\n", r.diagnosticPrefix(), builtin, value)
			return false
		}
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

func (r Runtime) completionUsage(builtin, message string) {
	if message != "" {
		fmt.Fprintf(r.streams.Stderr, "%s%s: %s\n", r.diagnosticPrefix(), builtin, message)
	}
	fmt.Fprintln(r.streams.Stderr, completionUsages[builtin])
}
