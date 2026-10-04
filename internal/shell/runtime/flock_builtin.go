package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// flock is busybox's applet (util-linux/flock.c), a builtin here because both its forms need the
// shell: `flock [-sxun] FD` locks a descriptor the shell opened, `exec 9>lock; flock -n 9`, and
// `flock [-sxn] FILE [-c] PROG ARGS` runs a command while FILE is locked, a function or a
// builtin too, or with -c one line of shell text, as `sh -c` would run it. -x is the default;
// -s shares the lock; -u lets it go; -n answers 1 at once, saying nothing, rather than waiting
// for a lock someone else holds. A wait ends as Ctrl-C ends a command.
//
// The lock is one byte far past any data, on Windows; see flock_windows.go for why that is
// not busybox-w32's. It was not here: Windows has no program of the name.
func (r Runtime) flockBuiltin(ctx context.Context, args []string) int {
	request, operands, err := parseFlockArgs(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sflock: %v\n", r.diagnosticPrefix(), err)
		return 1
	}
	if len(operands) == 1 {
		return r.flockDescriptor(ctx, request, operands[0])
	}
	file, err := openLockFile(r, operands[0])
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sflock: cannot open '%s': %s\n", r.diagnosticPrefix(), operands[0], applets.CauseText(err))
		return 1
	}
	defer file.Close()
	if status, locked := r.applyLock(ctx, request, file); !locked {
		return status
	}
	command := operands[1:]
	if command[0] == "-c" || command[0] == "--command" {
		if len(command) != 2 {
			fmt.Fprintf(r.streams.Stderr, "%sflock: -c takes only one argument\n", r.diagnosticPrefix())
			return 1
		}
		return r.runShellText(ctx, command[1])
	}
	return r.runCommandResolved(ctx, command, true)
}

// flockRequest is what flock's options asked for.
type flockRequest struct {
	shared, unlock, nonBlocking bool
}

// parseFlockArgs reads the options up to the first word that is not one, as busybox's "+"
// stops there, so `flock -n f cmd -x` passes -x to cmd.
func parseFlockArgs(args []string) (flockRequest, []string, error) {
	var request flockRequest
	long := map[string]byte{"shared": 's', "exclusive": 'x', "unlock": 'u', "nonblock": 'n'}
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		option := args[0]
		args = args[1:]
		if option == "--" {
			break
		}
		letters := option[1:]
		if strings.HasPrefix(option, "--") {
			letter, known := long[option[2:]]
			if !known {
				return request, nil, fmt.Errorf("unrecognized option '%s'", option)
			}
			letters = string(letter)
		}
		for _, letter := range letters {
			switch letter {
			case 's':
				request.shared = true
			case 'x':
				request.shared = false
			case 'u':
				request.unlock = true
			case 'n':
				request.nonBlocking = true
			default:
				return request, nil, fmt.Errorf("unknown option -- %c", letter)
			}
		}
	}
	if len(args) == 0 {
		return request, nil, errors.New("expected a descriptor, or a FILE and a command to run")
	}
	return request, args, nil
}

// flockDescriptor is the FD form: the file a descriptor the shell holds was opened on.
func (r Runtime) flockDescriptor(ctx context.Context, request flockRequest, operand string) int {
	fd, err := strconv.Atoi(operand)
	if err != nil || fd < 0 {
		fmt.Fprintf(r.streams.Stderr, "%sflock: invalid number '%s'\n", r.diagnosticPrefix(), operand)
		return 1
	}
	file, err := r.fds.hostFile(fd)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sflock: %d: Bad file descriptor\n", r.diagnosticPrefix(), fd)
		return 1
	}
	status, _ := r.applyLock(ctx, request, file)
	return status
}

// applyLock locks file, or unlocks it under -u, and reports whether the command may run.
func (r Runtime) applyLock(ctx context.Context, request flockRequest, file *os.File) (int, bool) {
	if request.unlock {
		if err := unlockHostFile(file); err != nil {
			fmt.Fprintf(r.streams.Stderr, "%sflock: %s\n", r.diagnosticPrefix(), applets.CauseText(err))
			return 1, false
		}
		return 0, true
	}
	locked, err := lockHostFile(ctx, file, request.shared, !request.nonBlocking)
	switch {
	case err != nil && ctx.Err() != nil:
		return contextStatus(ctx), false
	case err != nil:
		fmt.Fprintf(r.streams.Stderr, "%sflock: %s\n", r.diagnosticPrefix(), applets.CauseText(err))
		return 1, false
	case !locked:
		return 1, false
	}
	return 0, true
}

// openLockFile opens FILE as busybox's does: read-only, made when it is not there, and a
// directory too.
func openLockFile(r Runtime, operand string) (*os.File, error) {
	resolved, err := r.ResolveNemoshPath(operand)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(resolved.Native, os.O_RDONLY|os.O_CREATE, r.createMode(0o666))
	if err != nil {
		if info, statErr := os.Stat(resolved.Native); statErr == nil && info.IsDir() {
			return os.Open(resolved.Native)
		}
	}
	return file, err
}

// hostFile is the file a descriptor was opened on: one `exec 9>lock` opened.
func (t *fdTable) hostFile(fd int) (*os.File, error) {
	entry, err := t.lookup(fd)
	if err != nil {
		return nil, err
	}
	for _, candidate := range []any{entry.description.closer, entry.description.writer, entry.description.reader} {
		if file, ok := candidate.(*os.File); ok {
			return file, nil
		}
	}
	return nil, errors.New("not a file")
}

// runShellText runs text as `sh -c` runs it, in a subshell, and answers its status: what
// busybox's applets reach through system(), watch's command and flock's -c.
func (r Runtime) runShellText(ctx context.Context, text string) int {
	child, err := r.subshellSnapshot(ctx)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
		return 1
	}
	status, _ := child.runScriptResult(ctx, text, r.currentLine(), false, 0)
	status = child.runOwnExitTrap(ctx, r.traps[trapExit], status)
	child.jobScope.cancelAndDrain()
	if err := child.fds.closeAll(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "%s%v\n", r.diagnosticPrefix(), err)
	}
	return status
}
