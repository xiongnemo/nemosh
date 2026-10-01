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
	// instructions is commands flattened, which is what branching needs: a label
	// can sit inside a block a jump comes from outside, and a tree has nowhere
	// for such a jump to land. See sed_flow.go.
	instructions []*sedCommand
	labels       map[string]int
	// quiet is -n: the pattern space is not printed at the end of the script, so
	// only an explicit p writes anything.
	quiet bool
	// binary is -b, busybox-w32's: a carriage return is kept as part of its line.
	binary bool
	// view is the process the run is in, which resolves a file command's FILE.
	view ProcessView
}

type sedCommand struct {
	address sedAddress
	// action is one of 'p', 'd', 'q', 's', 'y', '=', 'a', 'i', 'c', or '{' for
	// a block.
	action     byte
	substitute sedSubstitute
	translate  sedTranslate
	// text is the argument of a, i and c.
	text string
	// label is the name on `:`, `b`, `t` and `T`; jump is where a branch lands,
	// or the instruction past a block's body.
	label string
	jump  int
	// block is the command list of a `{...}` group, run under this command's
	// address.
	block []*sedCommand
	// file is the FILE of r and w, and writeTo is w's, opened when the run starts; see
	// sed_files.go.
	file    string
	writeTo *sedWriteFile
}

// sedSupportedCommands are the actions this build implements. What is left out is GNU's
// `first~step` addresses and its R, W, e and z.
const sedSupportedCommands = "pdqsy={aichHgGxnNPDbtT:rwl"

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
	if err := flattenSedProgram(program); err != nil {
		return nil, err
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
	case 'a', 'i', 'c':
		text, remainder := parseSedTextCommand(rest)
		return &sedCommand{address: address, action: action, text: text}, remainder, nil
	case ':', 'b', 't', 'T':
		// A label runs to the end of the command, and `;` does end it -- unlike
		// the text commands, where a `;` is text. That is what lets `:a;N;$!ba`
		// be written on one line.
		label, remainder := parseSedLabel(rest[1:])
		return &sedCommand{address: address, action: action, label: label}, remainder, nil
	case 'r', 'w':
		if action == 'r' && address.ranged {
			return nil, "", fmt.Errorf("command 'r' uses only one address")
		}
		name, remainder, err := parseSedFileName(rest[1:])
		if err != nil {
			return nil, "", err
		}
		return &sedCommand{address: address, action: action, file: name}, remainder, nil
	}
	// p, d, q and = take no argument, so whatever follows is the next command.
	return &sedCommand{address: address, action: action}, rest[1:], nil
}

// parseSedLabel reads the name after `:`, `b`, `t` or `T`.
//
// Leading blanks are separators and a `;` or newline ends it, which is what makes
// `:a;N;$!ba` a one-liner. A label may be absent on a branch, where it means the
// end of the script.
func parseSedLabel(rest string) (string, string) {
	rest = strings.TrimLeft(rest, " \t")
	end := 0
	for end < len(rest) && rest[end] != ';' && rest[end] != '\n' && rest[end] != '}' {
		end++
	}
	return strings.TrimRight(rest[:end], " \t"), rest[end:]
}
