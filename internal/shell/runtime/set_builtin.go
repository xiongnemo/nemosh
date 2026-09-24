package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// shellOptionLine is how both option listings print a row, and it is one constant because the
// two were not the same: `shopt` padded the name to twelve and then added a tab, `set -o`
// padded to twelve and added nothing at all. Two listings of the same shell's own options did
// not line up with each other. bash pads to fifteen and adds a tab; twelve fits every name
// here, and the tab is what bash and busybox both put there.
const shellOptionLine = "%-12s\t%s\n"

// set implements the POSIX `set` builtin. With no arguments it lists the shell
// variables; `-o` or `+o` with no name lists the options; a `-` or `+` followed
// by letters, or by `o name`, turns those options on and off; and whatever is
// left after the options -- or a bare `--` -- replaces the positional
// parameters.
//
// An unknown option is refused rather than ignored, with busybox ash's wording
// and status (`illegal option -%c`, shell/ash.c:2433, and exitstatus 2 from
// ash_msg_and_raise_error, shell/ash.c:1803). Accepting one silently is worse
// than failing: a script asking for a flag this shell does not have would
// otherwise run on believing it had the protection.
func (r Runtime) set(args []string) int {
	if len(args) == 0 {
		return r.listShellVariables()
	}
	index, replacePositional, status := r.applySetOptions(args)
	if status != 0 {
		return status
	}
	if index < len(args) || replacePositional {
		r.params.values = append(r.params.values[:0], args[index:]...)
	}
	return 0
}

// applySetOptions consumes the leading option arguments and reports where the
// operands start, whether the positional parameters must be replaced even when
// there are none, and any failure. `set -e` alone must leave $1... alone, which
// is why the second answer is not just "are there operands".
func (r Runtime) applySetOptions(args []string) (int, bool, int) {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			return index + 1, true, 0
		}
		// The obsolescent `set -` of POSIX XCU 2.14: end option processing and
		// turn off xtrace and verbose, which is what dash does with it.
		if arg == "-" {
			r.options.xtrace, r.options.verbose = false, false
			return index + 1, true, 0
		}
		if len(arg) < 2 || arg[0] != '-' && arg[0] != '+' {
			return index, false, 0
		}
		enable := arg[0] == '-'
		if arg[1:] != "o" {
			if status := r.setLetterOptions(arg[1:], enable); status != 0 {
				return index, false, status
			}
			continue
		}
		if index+1 == len(args) {
			r.listShellOptions(enable)
			continue
		}
		index++
		if status := r.setNamedOption(args[index], enable); status != 0 {
			return index, false, status
		}
	}
	return len(args), false, 0
}

func (r Runtime) setLetterOptions(letters string, enable bool) int {
	for index := 0; index < len(letters); index++ {
		if err := r.setOptionLetter(letters[index], enable); err != nil {
			fmt.Fprintf(r.streams.Stderr, "set: %v\n", err)
			return 2
		}
	}
	return 0
}

func (r Runtime) setNamedOption(name string, enable bool) int {
	if err := r.setOptionName(name, enable); err != nil {
		fmt.Fprintf(r.streams.Stderr, "set: %v\n", err)
		return 2
	}
	return 0
}

// setOptionLetter and setOptionName are `set` without its diagnostics, which is what
// nemosh's own command line uses too (invocation_options.go): the error says what was
// wrong, and each caller says who it was.
func (r Runtime) setOptionLetter(letter byte, enable bool) error {
	flag, ok := r.options.byLetter(letter)
	if !ok {
		return fmt.Errorf("%w %c%c", ErrUnknownOption, optionSign(enable), letter)
	}
	if err := inertOptionRefusal(letter, enable); err != nil {
		return err
	}
	*flag = enable
	return nil
}

func (r Runtime) setOptionName(name string, enable bool) error {
	spec, ok := shellOptionSpecByName(name)
	if !ok {
		return fmt.Errorf("%w %co %s", ErrUnknownOption, optionSign(enable), name)
	}
	if err := inertOptionRefusal(spec.letter, enable); err != nil {
		return err
	}
	if reason, inert := inertShellOptionNames[name]; inert && enable {
		return fmt.Errorf("-o %s: not implemented: %s", name, reason)
	}
	*spec.field(r.options) = enable
	return nil
}

// inertOptionRefusal stops an option that would be remembered and never read.
// Turning one *off* is always allowed, because that is the state it is already
// in; only asking for behaviour that does not exist is refused.
//
// The alternative -- storing it and reporting it through `$-` -- was what this
// shell did until every other option was made to act, and it is the same shape
// of lie as an applet swallowing a flag: the script goes on believing it asked
// for something.
func inertOptionRefusal(letter byte, enable bool) error {
	if !enable {
		return nil
	}
	reason, inert := inertShellOptions[letter]
	if !inert {
		return nil
	}
	return fmt.Errorf("-%c: not implemented: %s", letter, reason)
}

var inertShellOptions = map[byte]string{
	'b': "asynchronous job completion is reported when `wait` or `jobs` asks, " +
		"not the moment it happens; there is no notification channel to switch on",
	'n': "a script is parsed in full before any of it runs, so by the time this " +
		"option is set there is no unread input left to withhold; " +
		"`nemosh -n SCRIPT` is the syntax check",
	'v': "a script is parsed in full before any of it runs, so there is no " +
		"moment at which its lines are read one by one to be echoed",
	'm': "there is no job control to switch on: nothing here can stop a job " +
		"and resume it, which is what fg and bg are refused for too",
}

// inertShellOptionNames is the same for options that have only a name. busybox accepts
// both of these; asking for either was "illegal option", which says the option does not
// exist rather than that this shell cannot do it.
var inertShellOptionNames = map[string]string{
	"vi": "the line editor's keys are emacs's, and there is no vi mode to switch to",
}

// Every other option acts: -a exports what is assigned (readonly.go), -C
// refuses to truncate (redirect_apply.go), -e leaves on failure and -u on an
// unset parameter (execute_pipeline.go, expansion_state.go), -f turns pathname
// expansion off (pathname_expansion.go), and -x traces commands (trace.go).

func optionSign(enable bool) byte {
	if enable {
		return '-'
	}
	return '+'
}

// `set -o` reports the states for a reader; `set +o` reports them as commands
// that recreate the same state when read back as input, which is what POSIX
// asks the `+o` form for.
func (r Runtime) listShellOptions(long bool) {
	for _, spec := range shellOptionSpecs {
		enabled := *spec.field(r.options)
		if long {
			state := "off"
			if enabled {
				state = "on"
			}
			fmt.Fprintf(r.streams.Stdout, shellOptionLine, spec.name, state)
			continue
		}
		sign := "+o"
		if enabled {
			sign = "-o"
		}
		fmt.Fprintf(r.streams.Stdout, "set %s %s\n", sign, spec.name)
	}
}

// The listing is single-quoted so it can be read back in, matching what
// busybox's showvars does through single_quote (libbb/).
func (r Runtime) listShellVariables() int {
	for _, name := range slices.Sorted(maps.Keys(r.vars)) {
		fmt.Fprintf(r.streams.Stdout, "%s=%s\n", name, singleQuoteForReuse(r.vars[name]))
	}
	return 0
}

func singleQuoteForReuse(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
