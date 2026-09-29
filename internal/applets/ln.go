package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
)

// ln is busybox-w32's (coreutils/ln.c): `[-sfnbv] [-S SUF] [-T] TARGET... LINK|DIR`.
//
// It took -s alone and exactly two operands, so `ln -sf target link` -- the way a script
// replaces a link -- failed on the option, and so did `ln -f a b`, `ln a b dir` and `ln
// dir/file`. What busybox does, measured:
//
//   - one operand links it into the working directory under its last component;
//   - several operands, or a last operand that is a directory, link each into that
//     directory under its own last component;
//   - -f removes a link name that is there, -b renames it to NAME~ first (-S sets the
//     suffix), -v says `'link' -> 'target'` for each;
//   - -n does not follow a last operand that is a symbolic link to a directory, and -T
//     takes the last operand as the link name whatever it is, and at most two operands;
//   - a target that cannot be linked is named and the rest are still linked, status 1.
func newLnApplet() Applet {
	return simpleApplet{name: "ln", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "sfnbvT", "S")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		if options.has('T') && len(operands) > 2 {
			return errors.New("-T accepts 2 args max")
		}
		last := operands[len(operands)-1]
		targets := operands[:len(operands)-1]
		if len(operands) == 1 {
			targets, last = operands, lastPathComponent(operands[0])
		}
		request := lnRequest{view: ProcessViewFromContext(ctx), options: options, stdout: stdout}
		failed := false
		for _, target := range targets {
			if err := request.link(target, last); err != nil {
				if len(targets) == 1 || !reportOperand(ctx, err) {
					return err
				}
				failed = true
			}
		}
		if failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

type lnRequest struct {
	view    ProcessView
	options appletOptions
	stdout  io.Writer
}

// link makes one link to target, named last or, when last is a directory, named for target
// inside it.
func (r lnRequest) link(target, last string) error {
	name := last
	directory, err := r.isDirectory(last)
	if err != nil {
		return err
	}
	if directory {
		if r.options.has('T') {
			return fmt.Errorf("'%s' is a directory", last)
		}
		name = strings.TrimRight(last, `/\`) + "/" + lastPathComponent(target)
	}
	host, err := resolveHostPath(r.view, name)
	if err != nil {
		return err
	}
	targetHost, err := resolveHostPath(r.view, target)
	if err != nil {
		return err
	}
	symbolic := r.options.has('s')
	if !symbolic {
		// A hard link needs a target that is there, or a symbolic link that is, dangling or not.
		if _, err := os.Lstat(targetHost); err != nil {
			return operandFailure(target, err)
		}
	}
	if err := r.clearName(host, name); err != nil {
		return err
	}
	if r.options.has('v') {
		fmt.Fprintf(r.stdout, "'%s' -> '%s'\n", name, target)
	}
	if symbolic {
		// A symbolic link holds its target as written, to be read from where the link is.
		err = os.Symlink(target, host)
	} else {
		err = os.Link(targetHost, host)
	}
	if err != nil {
		return operandFailure(name, err)
	}
	return nil
}

// clearName makes way for the link under -b or -f, as busybox does: -b renames what is there
// to NAME~ (or -S's suffix), and -f removes it. Nothing being there is no failure.
func (r lnRequest) clearName(host, name string) error {
	switch {
	case r.options.has('b'):
		suffix := "~"
		if r.options.has('S') {
			suffix = r.options.value('S')
		}
		if err := os.Rename(host, host+suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return operandFailure(name, err)
		}
		os.Remove(host)
	case r.options.has('f'):
		os.Remove(host)
	}
	return nil
}

// isDirectory reports whether the last operand is a directory to link into. A symbolic link to
// one counts unless -n or -T says to take the link itself.
func (r lnRequest) isDirectory(last string) (bool, error) {
	host, err := resolveHostPath(r.view, last)
	if err != nil {
		return false, err
	}
	stat := os.Stat
	if r.options.has('n') || r.options.has('T') {
		stat = os.Lstat
	}
	info, err := stat(host)
	return err == nil && info.IsDir(), nil
}

// lastPathComponent is an operand's last component, trailing separators dropped, as busybox's
// bb_get_last_path_component_strip has it: `dir/file` and `dir/file/` are both file.
func lastPathComponent(operand string) string {
	trimmed := strings.TrimRight(operand, `/\`)
	if trimmed == "" {
		return operand
	}
	return trimmed[strings.LastIndexAny(trimmed, `/\`)+1:]
}
