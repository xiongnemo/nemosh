package applets

import "fmt"

// appletOptions is what an applet's leading flags parsed to: present reports
// whether a flag was given, value carries the argument of the ones that take
// one.
type appletOptions struct {
	given  map[byte]bool
	values map[byte]string
}

func (o appletOptions) has(letter byte) bool { return o.given[letter] }

func (o appletOptions) value(letter byte) string { return o.values[letter] }

// parseAppletOptions splits the leading options off an applet's arguments.
// `flags` lists the letters that stand alone, `valued` the ones that take the
// rest of their word or the next argument. Clustered letters are the same as
// separate ones, a `--` ends option parsing, and a lone `-` is an operand --
// which is what getopt does and what busybox inherits from it.
//
// An unrecognised letter is an error rather than an operand. Swallowing it, as
// `wc -z FILE` and `touch -z` used to, makes the applet quietly do something
// other than what it was asked, and `wc -z` even exited 0 while counting
// nothing. busybox reaches bb_show_usage here; Nemosh has no usage text and
// says so in one line instead, which is the divergence recorded in
// docs/design/v0-readiness.md.
func parseAppletOptions(args []string, flags, valued string) (appletOptions, []string, error) {
	parsed := appletOptions{given: map[byte]bool{}, values: map[byte]string{}}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			return parsed, args[index+1:], nil
		}
		if len(arg) < 2 || arg[0] != '-' {
			return parsed, args[index:], nil
		}
		for position := 1; position < len(arg); position++ {
			letter := arg[position]
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
				index += consumed
				position = len(arg)
			default:
				return parsed, nil, invalidOption(letter)
			}
		}
	}
	return parsed, nil, nil
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

// twoOperands is the source-and-destination shape cp and mv take. A count that
// is not two used to fail silently, so `cp one.txt` and `cp a b c` both exited
// 1 with nothing said about which of them was wrong.
func twoOperands(args []string) ([]string, error) {
	_, operands, err := parseAppletOptions(args, "", "")
	if err != nil {
		return nil, err
	}
	switch {
	case len(operands) < 2:
		return nil, missingOperand()
	case len(operands) > 2:
		return nil, fmt.Errorf("extra operand '%s'", operands[2])
	}
	return operands, nil
}

func containsByte(set string, letter byte) bool {
	for index := 0; index < len(set); index++ {
		if set[index] == letter {
			return true
		}
	}
	return false
}
