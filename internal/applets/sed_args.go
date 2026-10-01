package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// sed's command line: its options, and the script files -f names.

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
	binary  bool
}

// sedArgs reads sed's options, leaving the operands.
//
// -n, -e, -E, -r, -f, -i and -b. -f collects a script from a file, so it is resolved
// here rather than at parse time: a missing script file is an error about that
// file, not about a script.
func sedArgs(ctx context.Context, args []string, stdin io.Reader) (sedOptions, error) {
	var options sedOptions
	// Options may follow the script and the files, `sed s/a/b/ f -n`, as getopt lets them.
	permute := optionsPermute(ProcessViewFromContext(ctx))
	var operands []string
	index := 0
	for ; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			if !permute {
				break
			}
			operands = append(operands, arg)
			continue
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
		consumed, err := readSedFlags(ctx, arg, args, index, &options, stdin)
		if err != nil {
			return sedOptions{}, err
		}
		index += consumed - 1
	}
	operands = append(operands, args[index:]...)
	if len(options.scripts) == 0 {
		if len(operands) == 0 {
			return sedOptions{}, missingOperand()
		}
		options.scripts = append(options.scripts, operands[0])
		operands = operands[1:]
	}
	options.operands = operands
	return options, nil
}

// readSedFlags reads one argument's worth of clustered letters, reporting how
// many arguments it used.
func readSedFlags(ctx context.Context, arg string, args []string, index int, options *sedOptions, stdin io.Reader) (int, error) {
	for position := 1; position < len(arg); position++ {
		switch letter := arg[position]; letter {
		case 'n':
			options.quiet = true
		case 'E', 'r':
			options.extended = true
		case 'b':
			options.binary = true
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
			script, err := readSedScriptFile(ctx, path, stdin)
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
// exactly as a `;`-separated script's are. `-f -` is standard input, as busybox's
// xfopen_stdin reads it; it was a file named -.
func readSedScriptFile(ctx context.Context, path string, stdin io.Reader) (string, error) {
	file := io.NopCloser(stdin)
	if path != "-" {
		opened, err := OpenProcessInput(ctx, ProcessViewFromContext(ctx), path)
		if err != nil {
			return "", operandFailure(path, err)
		}
		file = opened
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return "", operandFailure(path, err)
	}
	return string(data), nil
}
