package applets

import (
	"context"
	"fmt"
	"strings"
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
	return readAppletOptions(args, flags, valued, nil, optionsPermute(ProcessViewFromContext(ctx)))
}

// parseAppletLongOptions is parseAppletOptions for an applet with long options too: long maps
// each to the letter it stands for. See readLongOption.
func parseAppletLongOptions(ctx context.Context, args []string, long map[string]string, flags, valued string) (appletOptions, []string, error) {
	return readAppletOptions(args, flags, valued, long, optionsPermute(ProcessViewFromContext(ctx)))
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
	return readAppletOptions(args, flags, valued, nil, false)
}

// parseAppletLongOptionsInOrder is parseAppletOptionsInOrder with long options too, which end at
// the first operand as the letters do: what follows is a program's own.
func parseAppletLongOptionsInOrder(args []string, long map[string]string, flags, valued string) (appletOptions, []string, error) {
	return readAppletOptions(args, flags, valued, long, false)
}

func readAppletOptions(args []string, flags, valued string, long map[string]string, permute bool) (appletOptions, []string, error) {
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
		// A long option an applet does not have was read as letters, `du --apparent-size` as
		// `-`, and refused as that.
		if strings.HasPrefix(arg, "--") {
			used, err := parsed.readLongOption(args, index, flags, valued, long)
			if err != nil {
				return parsed, nil, err
			}
			index += used
			continue
		}
		for position := 1; position < len(arg); position++ {
			letter := arg[position]
			switch {
			case containsByte(flags, letter):
				parsed.set(letter)
			case containsByte(valued, letter):
				value, consumed, err := optionArgument(args, index, arg, position, letter)
				if err != nil {
					return parsed, nil, err
				}
				parsed.setValue(letter, value)
				index += consumed
				position = len(arg)
			default:
				return parsed, nil, invalidOption(letter)
			}
		}
	}
	return parsed, operands, nil
}

func (o *appletOptions) set(letter byte) {
	o.order = append(o.order, letter)
	o.given[letter] = true
}

func (o *appletOptions) setValue(letter byte, value string) {
	o.set(letter)
	o.values[letter] = value
	o.every[letter] = append(o.every[letter], value)
}

// readLongOption reads args[index] as getopt_long reads a long option: by its whole name or
// any prefix that names one letter alone, with its value after `=` or, for one that takes a
// value, in the next argument, which it reports it used. Long options were each turned into
// their letter before any option was read, so one in the arguments of a program env runs was
// rewritten, `env echo --null x` echoing -0 x; a value given one that takes none became an
// operand, `mkdir --parents=yes d` making yes; and a missing value was named by the letter.
func (o *appletOptions) readLongOption(args []string, index int, flags, valued string, long map[string]string) (int, error) {
	given, value, attached := strings.Cut(args[index][2:], "=")
	letter, err := longOptionLetter(long, given)
	switch {
	case err != nil:
		return 0, err
	case letter == 0 || !containsByte(flags+valued, letter):
		return 0, unknownLongOption(args[index])
	case containsByte(flags, letter) && attached:
		return 0, optionTakesNoArgument(given)
	case containsByte(flags, letter):
		o.set(letter)
		return 0, nil
	case attached:
		o.setValue(letter, value)
		return 0, nil
	case index+1 >= len(args):
		return 0, missingOptionArgument(given)
	}
	o.setValue(letter, args[index+1])
	return 1, nil
}

// longOptionLetter is the letter of the long option given, whole or by a prefix of names that
// all stand for one letter, or 0 for none, as getopt_long takes `--par` for --parents in every
// busybox applet with long options. A prefix two letters share is ambiguous.
func longOptionLetter(names map[string]string, given string) (byte, error) {
	if letter, known := names[given]; known {
		return letter[0], nil
	}
	found := ""
	for name, letter := range names {
		if given == "" || !strings.HasPrefix(name, given) {
			continue
		}
		if found != "" && found != letter {
			return 0, ambiguousOption(given)
		}
		found = letter
	}
	if found == "" {
		return 0, nil
	}
	return found[0], nil
}

// optionArgument takes the rest of the word if there is any -- `-m755` -- and
// otherwise the next argument, reporting how many extra arguments it used.
func optionArgument(args []string, index int, arg string, position int, letter byte) (string, int, error) {
	if position+1 < len(arg) {
		return arg[position+1:], 0, nil
	}
	if index+1 >= len(args) {
		return "", 0, missingOptionArgument(string(letter))
	}
	return args[index+1], 1, nil
}

// What getopt says of an option it cannot take, in the words both references say it on
// Windows: busybox-w32's getopt is the mingw runtime's and MSYS's GNU tools have Cygwin's, and
// both are NetBSD's. A long option is named as it was typed, with any value it was given.
// These were glibc's, `invalid option -- 'x'` and `unrecognized option '--x'`, which neither
// says here.
func invalidOption[Letter byte | rune](letter Letter) error {
	return fmt.Errorf("unknown option -- %c", letter)
}

func unknownLongOption(arg string) error {
	return fmt.Errorf("unknown option -- %s", strings.TrimPrefix(arg, "--"))
}

func missingOptionArgument(name string) error {
	return fmt.Errorf("option requires an argument -- %s", name)
}

func ambiguousOption(given string) error {
	return fmt.Errorf("ambiguous option -- %s", given)
}

func optionTakesNoArgument(given string) error {
	return fmt.Errorf("option does not take an argument -- %s", given)
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
