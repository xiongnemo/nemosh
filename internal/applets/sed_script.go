package applets

import (
	"fmt"
	"strings"
)

// The script: a list of addressed commands, parsed whole before any line is
// read.
//
// That ordering is the same rule find follows, and for the same reason: a caller
// piping sed's output into something else must not receive half an answer before
// the script turns out to be unusable.

// sedProgram is a parsed script together with the options that shape its output.
type sedProgram struct {
	commands []*sedCommand
	// quiet is -n: the pattern space is not printed at the end of the script, so
	// only an explicit p writes anything.
	quiet bool
}

type sedCommand struct {
	address sedAddress
	// action is one of 'p', 'd', 'q', 's'.
	action     byte
	substitute sedSubstitute
}

// sedSupportedCommands are the actions this build implements. Everything else is
// refused by name, including the ones busybox has: a/i/c need a text argument
// with its own continuation rules, y/// needs a transliteration table, and the
// hold-space commands need a second buffer. Answering any of them by guessing
// would be worse than saying so.
const sedSupportedCommands = "pdqs"

// parseSedProgram reads every -e script, and the first operand when there was
// no -e.
func parseSedProgram(scripts []string, quiet, extended bool) (*sedProgram, error) {
	program := &sedProgram{quiet: quiet}
	for _, script := range scripts {
		if err := program.parseScript(script, extended); err != nil {
			return nil, err
		}
	}
	if len(program.commands) == 0 {
		return nil, fmt.Errorf("no script")
	}
	return program, nil
}

// parseScript reads one script, whose commands are separated by `;` or a
// newline.
//
// The separator is found by walking rather than by strings.Split, because a `;`
// inside `s/a;b/x/` is part of the pattern -- splitting first would cut the
// substitution in half and report a malformed one.
func (p *sedProgram) parseScript(script string, extended bool) error {
	rest := strings.TrimLeft(script, " \t\n;")
	for rest != "" {
		command, remainder, err := parseSedCommand(rest, extended)
		if err != nil {
			return err
		}
		p.commands = append(p.commands, command)
		rest = strings.TrimLeft(remainder, " \t\n;")
	}
	return nil
}

func parseSedCommand(script string, extended bool) (*sedCommand, string, error) {
	address, rest, err := parseSedAddress(script, extended)
	if err != nil {
		return nil, "", err
	}
	rest = strings.TrimLeft(rest, " \t")
	if rest == "" {
		return nil, "", fmt.Errorf("missing command")
	}
	action := rest[0]
	if !strings.ContainsRune(sedSupportedCommands, rune(action)) {
		return nil, "", fmt.Errorf("unsupported command %s", string(action))
	}
	if action != 's' {
		// p, d and q take no argument, so whatever follows is the next command.
		return &sedCommand{address: address, action: action}, rest[1:], nil
	}
	substitute, remainder, err := parseSedSubstituteCommand(rest, extended)
	if err != nil {
		return nil, "", err
	}
	return &sedCommand{address: address, action: 's', substitute: substitute}, remainder, nil
}

// sedArgs reads sed's options, leaving the operands.
//
// -n, -e, -E and -r. -i and -f are refused: -i has to choose an *output*
// encoding for the file it rewrites, which is the same decision already deferred
// for sed's UTF-16 reading, and burying it inside a convenience flag would
// settle it by accident.
func sedArgs(args []string) (scripts []string, operands []string, quiet, extended bool, err error) {
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
		if strings.HasPrefix(arg, "--") {
			switch arg {
			case "--quiet", "--silent":
				quiet = true
				continue
			case "--regexp-extended":
				extended = true
				continue
			}
			return nil, nil, false, false, fmt.Errorf("unsupported sed option: %s", arg)
		}
		consumed, script, hasScript, optErr := readSedFlags(arg, args, index, &quiet, &extended)
		if optErr != nil {
			return nil, nil, false, false, optErr
		}
		if hasScript {
			scripts = append(scripts, script)
		}
		index += consumed - 1
	}
	if len(scripts) == 0 {
		if index >= len(args) {
			return nil, nil, false, false, missingOperand()
		}
		scripts = append(scripts, args[index])
		index++
	}
	return scripts, args[index:], quiet, extended, nil
}

// readSedFlags reads one argument's worth of clustered letters, reporting how
// many arguments it used and any script it collected.
func readSedFlags(arg string, args []string, index int, quiet, extended *bool) (int, string, bool, error) {
	for position := 1; position < len(arg); position++ {
		switch letter := arg[position]; letter {
		case 'n':
			*quiet = true
		case 'E', 'r':
			*extended = true
		case 'e':
			if position+1 < len(arg) {
				return 1, arg[position+1:], true, nil
			}
			if index+1 >= len(args) {
				return 0, "", false, fmt.Errorf("option requires an argument -- 'e'")
			}
			return 2, args[index+1], true, nil
		default:
			return 0, "", false, fmt.Errorf("unsupported sed option: -%c", letter)
		}
	}
	return 1, "", false, nil
}
