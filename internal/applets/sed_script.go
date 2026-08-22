package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	// action is one of 'p', 'd', 'q', 's', 'y', '=', or '{' for a block.
	action     byte
	substitute sedSubstitute
	translate  sedTranslate
	// block is the command list of a `{...}` group, run under this command's
	// address.
	block []*sedCommand
}

// sedSupportedCommands are the actions this build implements. Everything else is
// refused by name, including the ones busybox has: a/i/c need a text argument
// with its own continuation rules, and the hold-space commands need a second
// buffer. Answering any of them by guessing would be worse than saying so.
const sedSupportedCommands = "pdqsy={"

// parseSedProgram reads every -e script, and the first operand when there was
// no -e.
func parseSedProgram(scripts []string, quiet, extended bool) (*sedProgram, error) {
	program := &sedProgram{quiet: quiet}
	for _, script := range scripts {
		if err := program.parseScript(script, extended); err != nil {
			return nil, err
		}
	}
	// An empty script is a valid no-op, not an error: `sed '' file` copies the
	// file and `sed -n '' file` prints nothing, which is what busybox does.
	// Refusing it made `sed "$expr" file` fail when the variable was empty,
	// where every reference passes the input through.
	return program, nil
}

// parseScript reads one script, whose commands are separated by `;` or a
// newline.
//
// The separator is found by walking rather than by strings.Split, because a `;`
// inside `s/a;b/x/` is part of the pattern -- splitting first would cut the
// substitution in half and report a malformed one.
func (p *sedProgram) parseScript(script string, extended bool) error {
	commands, rest, err := parseSedCommandList(script, extended, false)
	if err != nil {
		return err
	}
	if rest != "" {
		return fmt.Errorf("unexpected `}'")
	}
	p.commands = append(p.commands, commands...)
	return nil
}

// parseSedCommandList reads commands until the script runs out, or until the `}`
// that closes a block.
//
// inBlock says which of those two endings is expected, so an unclosed `{` and a
// stray `}` are told apart and each named.
func parseSedCommandList(script string, extended, inBlock bool) ([]*sedCommand, string, error) {
	var commands []*sedCommand
	rest := strings.TrimLeft(script, " \t\n;")
	for rest != "" {
		if rest[0] == '}' {
			if !inBlock {
				return nil, rest, nil
			}
			return commands, rest[1:], nil
		}
		command, remainder, err := parseSedCommand(rest, extended)
		if err != nil {
			return nil, "", err
		}
		commands = append(commands, command)
		rest = strings.TrimLeft(remainder, " \t\n;")
	}
	if inBlock {
		return nil, "", fmt.Errorf("unmatched `{'")
	}
	return commands, "", nil
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
	switch action {
	case '{':
		// A group under one address, which is what makes `/x/{p;q}` apply both
		// commands to the matching line and neither to any other.
		block, remainder, err := parseSedCommandList(rest[1:], extended, true)
		if err != nil {
			return nil, "", err
		}
		return &sedCommand{address: address, action: '{', block: block}, remainder, nil
	case 's':
		substitute, remainder, err := parseSedSubstituteCommand(rest, extended)
		if err != nil {
			return nil, "", err
		}
		return &sedCommand{address: address, action: 's', substitute: substitute}, remainder, nil
	case 'y':
		translate, remainder, err := parseSedTranslateCommand(rest)
		if err != nil {
			return nil, "", err
		}
		return &sedCommand{address: address, action: 'y', translate: translate}, remainder, nil
	}
	// p, d, q and = take no argument, so whatever follows is the next command.
	return &sedCommand{address: address, action: action}, rest[1:], nil
}

// sedOptions is what sed's flags parsed to.
type sedOptions struct {
	scripts  []string
	operands []string
	quiet    bool
	extended bool
	// inPlace is -i, and suffix is the backup suffix attached to it. inPlace has
	// to be separate from a non-empty suffix, because `-i` with no suffix is the
	// common form and keeps no backup.
	inPlace bool
	suffix  string
}

// sedArgs reads sed's options, leaving the operands.
//
// -n, -e, -E, -r, -f and -i. -f collects a script from a file, so it is resolved
// here rather than at parse time: a missing script file is an error about that
// file, not about a script.
func sedArgs(ctx context.Context, args []string) (sedOptions, error) {
	var options sedOptions
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
				options.quiet = true
				continue
			case "--regexp-extended":
				options.extended = true
				continue
			case "--in-place":
				options.inPlace = true
				continue
			}
			return sedOptions{}, fmt.Errorf("unsupported sed option: %s", arg)
		}
		consumed, err := readSedFlags(ctx, arg, args, index, &options)
		if err != nil {
			return sedOptions{}, err
		}
		index += consumed - 1
	}
	if len(options.scripts) == 0 {
		if index >= len(args) {
			return sedOptions{}, missingOperand()
		}
		options.scripts = append(options.scripts, args[index])
		index++
	}
	options.operands = args[index:]
	return options, nil
}

// readSedFlags reads one argument's worth of clustered letters, reporting how
// many arguments it used.
func readSedFlags(ctx context.Context, arg string, args []string, index int, options *sedOptions) (int, error) {
	for position := 1; position < len(arg); position++ {
		switch letter := arg[position]; letter {
		case 'n':
			options.quiet = true
		case 'E', 'r':
			options.extended = true
		case 'i':
			// The suffix is attached and never a separate word, which is what
			// keeps `sed -i script file` from taking the script as a suffix.
			// That is GNU's rule and the reason -i is spelled `-i.bak`.
			options.inPlace = true
			options.suffix = arg[position+1:]
			return 1, nil
		case 'e':
			script, consumed, err := sedFlagValue(arg, args, index, position, 'e')
			if err != nil {
				return 0, err
			}
			options.scripts = append(options.scripts, script)
			return consumed, nil
		case 'f':
			path, consumed, err := sedFlagValue(arg, args, index, position, 'f')
			if err != nil {
				return 0, err
			}
			script, err := readSedScriptFile(ctx, path)
			if err != nil {
				return 0, err
			}
			options.scripts = append(options.scripts, script)
			return consumed, nil
		default:
			return 0, fmt.Errorf("unsupported sed option: -%c", letter)
		}
	}
	return 1, nil
}

// sedFlagValue is the rest of the word, or the next argument when the word ends
// at the letter.
func sedFlagValue(arg string, args []string, index, position int, letter byte) (string, int, error) {
	if position+1 < len(arg) {
		return arg[position+1:], 1, nil
	}
	if index+1 >= len(args) {
		return "", 0, fmt.Errorf("option requires an argument -- '%c'", letter)
	}
	return args[index+1], 2, nil
}

// readSedScriptFile is -f: the script comes from a file, whose lines are commands
// exactly as a `;`-separated script's are.
func readSedScriptFile(ctx context.Context, path string) (string, error) {
	view := ProcessViewFromContext(ctx)
	file, err := OpenProcessInput(ctx, view, path)
	if err != nil {
		return "", operandFailure(path, err)
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return "", operandFailure(path, err)
	}
	return string(data), nil
}
