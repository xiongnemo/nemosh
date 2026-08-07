package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

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
		flag, ok := r.options.byLetter(letters[index])
		if !ok {
			fmt.Fprintf(r.streams.Stderr, "set: illegal option %c%c\n", optionSign(enable), letters[index])
			return 2
		}
		*flag = enable
	}
	return 0
}

func (r Runtime) setNamedOption(name string, enable bool) int {
	flag, ok := r.options.byName(name)
	if !ok {
		fmt.Fprintf(r.streams.Stderr, "set: illegal option %co %s\n", optionSign(enable), name)
		return 2
	}
	*flag = enable
	return 0
}

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
			fmt.Fprintf(r.streams.Stdout, "%-12s%s\n", spec.name, state)
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
