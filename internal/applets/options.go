package applets

import (
	"context"
	"fmt"
)

// appletOptions is what an applet's leading flags parsed to: present reports
// whether a flag was given, value carries the argument of the ones that take
// one.
type appletOptions struct {
	given  map[byte]bool
	values map[byte]string
	// every is each value a letter was given, in order, for an option that may be repeated.
	every map[byte][]string
	// order is the letters as they were given, for the options where the last one wins.
	order []byte
}

func (o appletOptions) has(letter byte) bool { return o.given[letter] }

func (o appletOptions) value(letter byte) string { return o.values[letter] }

func (o appletOptions) all(letter byte) []string { return o.every[letter] }

// last is whichever of letters was given last, or 0 when none was: of cp's -i and -n, the
// later one wins.
func (o appletOptions) last(letters string) byte {
	for index := len(o.order) - 1; index >= 0; index-- {
		if containsByte(letters, o.order[index]) {
			return o.order[index]
		}
	}
	return 0
}

// parseAppletOptions splits an applet's options from its operands. `flags` lists the letters
// that stand alone, `valued` the ones that take the rest of their word or the next argument.
// Clustered letters are the same as separate ones, a `--` ends option parsing, and a lone `-`
// is an operand -- which is what getopt does and what busybox inherits from it.
//
// Options may follow operands, `ls dir -l` as `ls -l dir`, as getopt permutes them for
// busybox's applets unless POSIXLY_CORRECT is set. They stopped at the first operand, so
// `rm f -v` tried to remove a file named -v, `touch t -c` made t, and `cp a b -v` wanted a
// directory named -v. An applet busybox reads in order, one whose operands are a command and
// its own arguments, uses parseAppletOptionsInOrder.
//
// An unrecognised letter is an error rather than an operand. Swallowing it, as
// `wc -z FILE` and `touch -z` used to, makes the applet quietly do something
// other than what it was asked, and `wc -z` even exited 0 while counting
// nothing. busybox reaches bb_show_usage here; Nemosh has no usage text and
// says so in one line instead, which is the divergence recorded in
// docs/design/v0-readiness.md.
func parseAppletOptions(ctx context.Context, args []string, flags, valued string) (appletOptions, []string, error) {
	return readAppletOptions(args, flags, valued, optionsPermute(ProcessViewFromContext(ctx)))
}

// optionsPermute is whether options may follow operands: unless POSIXLY_CORRECT is set.
func optionsPermute(view ProcessView) bool {
	if view == nil {
		return true
	}
	_, strict := view.LookupEnv("POSIXLY_CORRECT")
	return !strict
}

// parseAppletOptionsInOrder is parseAppletOptions for an applet whose options end at its first
// operand, as busybox's getopt string says with a leading `+`: `xargs echo -n` runs echo -n.
func parseAppletOptionsInOrder(args []string, flags, valued string) (appletOptions, []string, error) {
	return readAppletOptions(args, flags, valued, false)
}

func readAppletOptions(args []string, flags, valued string, permute bool) (appletOptions, []string, error) {
	parsed := appletOptions{given: map[byte]bool{}, values: map[byte]string{}, every: map[byte][]string{}}
	var operands []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			return parsed, append(operands, args[index+1:]...), nil
		}
		if len(arg) < 2 || arg[0] != '-' {
			if !permute {
				return parsed, append(operands, args[index:]...), nil
			}
			operands = append(operands, arg)
			continue
		}
		for position := 1; position < len(arg); position++ {
			letter := arg[position]
			parsed.order = append(parsed.order, letter)
			switch {
			case containsByte(flags, letter):
				parsed.given[letter] = true
			case containsByte(valued, letter):
				value, consumed, err := optionArgument(args, index, arg, position, letter)
				if err != nil {
					return parsed, nil, err
				}
				parsed.given[letter] = true
				parsed.values[letter] = value
				parsed.every[letter] = append(parsed.every[letter], value)
				index += consumed
				position = len(arg)
			default:
				return parsed, nil, invalidOption(letter)
			}
		}
	}
	return parsed, operands, nil
}

// optionArgument takes the rest of the word if there is any -- `-m755` -- and
// otherwise the next argument, reporting how many extra arguments it used.
func optionArgument(args []string, index int, arg string, position int, letter byte) (string, int, error) {
	if position+1 < len(arg) {
		return arg[position+1:], 0, nil
	}
	if index+1 >= len(args) {
		return "", 0, fmt.Errorf("option requires an argument -- '%c'", letter)
	}
	return args[index+1], 1, nil
}

func invalidOption(letter byte) error {
	return fmt.Errorf("invalid option -- '%c'", letter)
}

// twoOperandsWithOptions is the source-and-destination shape cp and mv take,
// once their own options are removed -- cp needs -r and mv needs -f. A count
// that is not two used to fail silently, so `cp one.txt` and `cp a b c` both
// exited 1 with nothing said about which of them was wrong.
//
// Three or more operands are no longer refused: POSIX gives both applets a
// `source_file... target_directory` form, and `cp a b c dir/` is common enough in
// scripts that refusing it with `extra operand 'c'` was the more surprising answer.
// The caller checks that the last operand is a directory; see copyManyOperands.
func twoOperandsWithOptions(ctx context.Context, args []string, short string) (appletOptions, []string, error) {
	options, operands, err := parseAppletOptions(ctx, args, short, "")
	if err != nil {
		return options, nil, err
	}
	if len(operands) < 2 {
		return options, nil, missingOperand()
	}
	return options, operands, nil
}

func containsByte(set string, letter byte) bool {
	for index := 0; index < len(set); index++ {
		if set[index] == letter {
			return true
		}
	}
	return false
}
