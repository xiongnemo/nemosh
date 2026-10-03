package runtime

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// setOptionLine is how the `set -o` listing prints a row: busybox's, padded to sixteen with
// no tab (ash.c's plus_minus_o). It was bash's shape, a tab after the name, which busybox does
// not write. Fifteen and a blank is the same sixteen for every name busybox has, and keeps a
// blank after one of bash's that is longer: `interactive-comments` ran into its `on`.
const setOptionLine = "%-15s %s\n"

// shellOptionLine is how `shopt -o` prints a row, bash's shape since busybox has no shopt: the
// name padded, then a tab. shopt, which has bash's names, has bash's width too; see shoptLine.
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
		r.params.getopts = getoptsState{}
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
		// turn off xtrace and verbose, which is what dash does with it. With nothing
		// after it the positional parameters stay, as busybox keeps them; they were
		// cleared.
		if arg == "-" {
			r.options.xtrace, r.options.verbose = false, false
			return index + 1, false, 0
		}
		// A lone `+` is an ignored flag, as busybox reads it: `set +` made `+` the one
		// positional parameter.
		if arg == "+" {
			continue
		}
		if len(arg) < 2 || arg[0] != '-' && arg[0] != '+' {
			return index, false, 0
		}
		enable := arg[0] == '-'
		// An `o` anywhere in the letters takes the next argument as its name, as in both
		// references: `set -euo pipefail`, the first line of a strict bash script. Only a
		// lone `-o` was understood, so that line was `illegal option -o` and status 2 --
		// under -e, the end of the script before it began.
		for _, letter := range []byte(arg[1:]) {
			if letter != 'o' {
				if status := r.setLetterOptions(string(letter), enable); status != 0 {
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
	}
	return len(args), false, 0
}

func (r Runtime) setLetterOptions(letters string, enable bool) int {
	for index := 0; index < len(letters); index++ {
		if err := r.setOptionLetter(letters[index], enable); err != nil {
			// A letter set has not got ends a script, as busybox's setoption raises it, where
			// a -o name it has not got is only 1; see setNamedOption. One it has and cannot
			// honour here is refused, and the script goes on.
			fmt.Fprintf(r.streams.Stderr, "%sset: %v\n", r.diagnosticPrefix(), err)
			if errors.Is(err, ErrUnknownOption) {
				r.raiseShellError()
			}
			return 2
		}
	}
	return 0
}

func (r Runtime) setNamedOption(name string, enable bool) int {
	if err := r.setOptionName(name, enable); err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sset: %v\n", r.diagnosticPrefix(), err)
		// A name set has not got is 1, as busybox's minus_o answers it, and the script goes on.
		if errors.Is(err, ErrUnknownOption) {
			return 1
		}
		return 2
	}
	return 0
}

// setOptionLetter and setOptionName are `set` without its diagnostics, which is what
// nemosh's own command line uses too (invocation_options.go): the error says what was
// wrong, and each caller says who it was.
func (r Runtime) setOptionLetter(letter byte, enable bool) error {
	spec, ok := shellOptionSpecByLetter(letter)
	if !ok {
		return fmt.Errorf("%w %c%c", ErrUnknownOption, optionSign(enable), letter)
	}
	if err := inertOptionRefusal(letter, enable); err != nil {
		return err
	}
	if err := fixedOptionRefusal(spec, enable); err != nil {
		return err
	}
	*spec.field(r.options) = enable
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
	if err := fixedOptionRefusal(spec, enable); err != nil {
		return err
	}
	*spec.field(r.options) = enable
	// vi and emacs are the line editor's two modes, and asking for one leaves the other, as
	// bash has them. busybox has only vi.
	if enable && (spec.name == "vi" || spec.name == "emacs") {
		r.options.vi, r.options.emacs = spec.name == "vi", spec.name == "emacs"
	}
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
	'v': "a script is parsed in full before any of it runs, so there is no " +
		"moment at which its lines are read one by one to be echoed",
	'm': "there is no job control to switch on: nothing here can stop a job " +
		"and resume it, which is what fg and bg are refused for too",
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
			fmt.Fprintf(r.streams.Stdout, setOptionLine, spec.name, state)
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
// busybox's showvars does through single_quote (libbb/). An array, which busybox
// has not got, is its compound value, as bash lists it: an indexed one was its
// element 0, and an associative one was left out. One declared and never given a
// key is left out, as bash leaves out a variable with no value.
func (r Runtime) listShellVariables() int {
	names := slices.Collect(maps.Keys(r.vars))
	if r.arrays != nil {
		for _, name := range r.arrays.associativeNames() {
			if len(r.arrays.keysOf(name)) > 0 {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		if list, ok := r.arrayLiteralText(name); ok {
			fmt.Fprintf(r.streams.Stdout, "%s=%s\n", name, list)
			continue
		}
		fmt.Fprintf(r.streams.Stdout, "%s=%s\n", name, shellquote.Ash(r.vars[name]))
	}
	return 0
}

// singleQuoteForReuse is a word in single quotes as bash writes one back out, a quote in it
// ending them, escaped, and opening them again, for printing a function as bash prints one.
// The ash family's own listings are shellquote.Ash.
func singleQuoteForReuse(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
