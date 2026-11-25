package applets

import (
	"context"
	"fmt"
	"io"
	"os"
)

// drop runs a command without elevated privileges: busybox-w32's (miscutils/drop.c), with its
// cdrop and pdrop, which run cmd.exe and PowerShell. With no command, or with -c, it runs a shell:
// this one for drop, or the one -s names, cmd.exe for cdrop and powershell.exe for pdrop.
//
//	drop  [COMMAND [ARG...] | [-s SHELL] -c CMD_STRING [ARG...]]
//	cdrop [COMMAND [ARG...] | -c CMD_STRING [ARG...]]
//
// The shell launches what it runs, with the token it is asked for through the context. Windows
// calls that token unelevated, where it still calls busybox's elevated. busybox's computes a
// "normal user" token, which leaves the group of local accounts that are administrators enabled,
// and TokenElevation reads 1; a program that refuses to run elevated, as Codex's app server does,
// refused busybox's drop too. See internal/shell/runtime/unelevated_token_windows.go.
//
// Its options end at the command, so `drop ls -l` runs ls -l. busybox's read on past it, and refused
// the -l as an option of drop's own.
type dropApplet struct{ name string }

func newDropApplet() Applet       { return dropApplet{name: "drop"} }
func newCdropApplet() Applet      { return dropApplet{name: "cdrop"} }
func newPdropApplet() Applet      { return dropApplet{name: "pdrop"} }
func (a dropApplet) Name() string { return a.name }

func (a dropApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	valued := "c"
	if a.name == "drop" {
		valued = "cs"
	}
	options, operands, err := parseAppletOptionsInOrder(args, "", valued)
	if err != nil {
		return err
	}
	program, programArgs, err := a.plan(ctx, options, operands)
	if err != nil {
		return err
	}
	ctx = WithProcessView(WithUnelevatedLaunch(ctx), droppedView(ProcessViewFromContext(ctx)))
	return program.Run(ctx, programArgs, stdin, stdout, stderr)
}

// plan is the program drop runs and its arguments: busybox's choices, drop.c:124-185.
func (a dropApplet) plan(ctx context.Context, options appletOptions, operands []string) (Applet, []string, error) {
	if len(operands) > 0 && !options.has('c') {
		name := operands[0]
		if _, ok := DefaultRegistry.Lookup(name); ok {
			return a.self(ctx, operands)
		}
		return a.find(ctx, name, operands[1:])
	}
	var tail []string
	if options.has('c') {
		tail = append([]string{"-c", options.value('c')}, operands...)
	}
	switch {
	case a.name == "cdrop":
		return a.find(ctx, "cmd.exe", cmdFlag(tail))
	case a.name == "pdrop":
		return a.find(ctx, "powershell.exe", tail)
	case options.has('s'):
		shell := options.value('s')
		if base := windowsBaseName(shell); base == "cmd.exe" || base == "cmd" {
			tail = cmdFlag(tail)
		}
		return a.find(ctx, shell, tail)
	}
	return a.self(ctx, tail)
}

// self is this program run with args, which is this shell, or an applet when args name one.
func (a dropApplet) self(ctx context.Context, args []string) (Applet, []string, error) {
	program, err := os.Executable()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot find this shell's own program: %w", err)
	}
	return a.find(ctx, program, args)
}

func (a dropApplet) find(ctx context.Context, name string, args []string) (Applet, []string, error) {
	program, ok := programFor(ctx, name)
	if !ok {
		return nil, nil, ExitStatusMessage(127, fmt.Errorf("can't find '%s'", name))
	}
	return program, args, nil
}

// cmdFlag is a shell's -c spelt as cmd.exe takes it.
func cmdFlag(tail []string) []string {
	if len(tail) > 0 && tail[0] == "-c" {
		tail = append([]string{"/c"}, tail[1:]...)
	}
	return tail
}

// droppedView is the environment drop gives what it runs: USER and LOGNAME the account's name where
// they are unset or root, the name this shell gives an elevated token, as busybox's setenv_name.
func droppedView(parent ProcessView) staticProcessView {
	view := newStaticProcessView(parent, parent.Environ())
	if account := processAccountName(); account != "" {
		for _, key := range []string{"USER", "LOGNAME"} {
			if value, ok := parent.LookupEnv(key); !ok || value == "root" {
				view.set(key, account)
			}
		}
	}
	return view
}
