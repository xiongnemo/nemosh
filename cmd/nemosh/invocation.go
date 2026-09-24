package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// invocation is nemosh's command line read the way sh reads its own, which busybox's usage
// line spells out:
//
//	nemosh [-ils] [-|+CEaefnux] [-|+o OPTION]... [-c COMMAND [NAME [ARG]...] | SCRIPT [ARG]...]
//
// Only -c and -i were understood, and only as the first argument. Every shell option was
// an invalid one, so `nemosh -e script` and `nemosh -x script` failed before reading a
// line. So did any script beginning `#!/bin/sh -e`, which the shell launches as
// `nemosh -e script` (external_script.go). busybox-w32 and bash accept all of them, and
// the letters mean what `set` makes them mean.
type invocation struct {
	command     bool // -c: the first operand is the script itself
	stdin       bool // -s: the script is standard input, and the operands are $1...
	interactive bool // -i
	login       bool // -l: /etc/profile and ~/.profile first, as a login shell reads them
	checkOnly   bool // -n: parse the script and run none of it
	listOptions bool // -o with nothing after it
	options     []invocationOption
	operands    []string
}

// invocationOption is one option for `set` to apply: a letter, or the name after -o.
type invocationOption struct {
	letter byte
	name   string
	enable bool
}

const optionHint = "hint: `nemosh --help` lists the options this build accepts"

var errInvalidOption = errors.New("invalid option")

func parseInvocation(args []string) (invocation, error) {
	var parsed invocation
	index := 0
	for ; index < len(args); index++ {
		arg := args[index]
		// `-` and `--` end the options, and both references step over either.
		if arg == "-" || arg == "--" {
			index++
			break
		}
		if strings.HasPrefix(arg, "--") {
			return parsed, fmt.Errorf("%w %s", errInvalidOption, arg)
		}
		if len(arg) < 2 || arg[0] != '-' && arg[0] != '+' {
			break
		}
		enable := arg[0] == '-'
		for _, letter := range []byte(arg[1:]) {
			switch {
			case letter != 'o':
				parsed.letter(letter, enable)
			case index+1 == len(args):
				parsed.listOptions = true
			default:
				index++
				parsed.named(args[index], enable)
			}
		}
	}
	parsed.operands = args[index:]
	if parsed.command && len(parsed.operands) == 0 {
		return parsed, errors.New("-c requires an argument")
	}
	if parsed.interactive && (parsed.command || !parsed.stdin && len(parsed.operands) > 0) {
		return parsed, errors.New("-i reads commands from the terminal, so it cannot also run -c or a script")
	}
	return parsed, nil
}

// letter sorts one option letter into the ones that say how to start and the ones for
// `set`. The first kind take only `-`: `+c` goes to `set`, which refuses it by name.
func (i *invocation) letter(letter byte, enable bool) {
	switch {
	case letter == 'n':
		i.checkOnly = enable
	case !enable || strings.IndexByte("csil", letter) < 0:
		i.options = append(i.options, invocationOption{letter: letter, enable: enable})
	case letter == 'c':
		i.command = true
	case letter == 's':
		i.stdin = true
	case letter == 'i':
		i.interactive = true
	default:
		i.login = true
	}
}

// named is -o NAME. noexec is -n under its long name.
func (i *invocation) named(name string, enable bool) {
	if name == "noexec" {
		i.checkOnly = enable
		return
	}
	i.options = append(i.options, invocationOption{name: name, enable: enable})
}

// session reports whether this invocation is a prompt. It is one when -i asks for it, or
// when commands come from standard input and that is a terminal: no -c and no script. That
// is busybox's rule (procargs in shell/ash.c), and bash's.
func (i invocation) session(stdinIsTerminal bool) bool {
	return i.interactive || stdinIsTerminal && !i.command && (i.stdin || len(i.operands) == 0)
}

// runInvocation starts the shell the command line describes.
func (c command) runInvocation(ctx context.Context, controller *interruptController, parsed invocation) error {
	c.invocation = parsed
	switch {
	case parsed.session(c.stdinIsTerminal):
		return c.runInteractive(ctx, controller)
	case parsed.command:
		return c.runScriptAs(ctx, controller, parsed.operands[0], commandStringInvocation(parsed.operands[1:]))
	case parsed.stdin || len(parsed.operands) == 0:
		data, err := readBoundedInput(c.stdin)
		if err != nil {
			if errors.Is(err, errInputTooLarge) {
				fmt.Fprintln(c.stderr, "nemosh: input too large")
				return exitStatus(2)
			}
			return fmt.Errorf("nemosh: read stdin: %w", err)
		}
		return c.runScriptAs(ctx, controller, string(data), scriptInvocation{name: defaultScriptName, args: parsed.operands, mode: "s"})
	}
	return c.runScriptFile(ctx, controller, parsed.operands[0], parsed.operands[1:])
}

// startShell prepares a runtime as the invocation asks, before anything runs: the options,
// `$-`, the listing a bare -o prints, and a login shell's profiles. A refused option has
// been reported when this returns, with status 2, which `set` gives for the same refusal.
func (c command) startShell(ctx context.Context, rt runtime.Runtime, mode string) error {
	for _, option := range c.invocation.options {
		if err := rt.SetOption(option.letter, option.name, option.enable); err != nil {
			fmt.Fprintf(c.stderr, "nemosh: %v\n", err)
			if errors.Is(err, runtime.ErrUnknownOption) {
				fmt.Fprintln(c.stderr, optionHint)
			}
			return exitStatus(2)
		}
	}
	rt.SetInvocationMode(mode)
	if c.invocation.listOptions {
		rt.ListOptions()
	}
	if c.invocation.login && !c.invocation.checkOnly {
		if status, exited := sourceLoginProfiles(ctx, rt, c.stderr); exited {
			return startupExit(rt, status)
		}
	}
	return nil
}

// startSession is startShell for a prompt, followed by $ENV. $0 is the shell's name, as
// it is in both references, and `-s ARG...` gives the session its positional parameters.
func (c command) startSession(ctx context.Context, rt runtime.Runtime) error {
	rt.SetArguments(defaultScriptName, c.invocation.operands)
	if err := c.startShell(ctx, rt, "is"); err != nil {
		return err
	}
	if status, exited := sourceStartupFile(ctx, rt, c.stderr); exited {
		return startupExit(rt, status)
	}
	return nil
}
