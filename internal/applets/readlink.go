package applets

import (
	"context"
	"fmt"
	"io"
	"os"
)

// readlink is busybox's (coreutils/readlink.c): `readlink [-fnvsq] FILE` prints what the
// symbolic link FILE points at. -f prints FILE's canonical path instead, as realpath does, with
// every link in it followed, and FILE itself need not be there so long as its directory is. -n
// leaves the newline off. A failure says nothing and answers 1, as busybox's does unless -v asks
// it why; -s and -q ask for the quiet that is already the default.
//
// -f was refused, and `readlink -f "$0"` is how a script commonly finds its own directory, so
// such a script ended at its first line.
func newReadlinkApplet() Applet {
	return simpleApplet{name: "readlink", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "nfvsq", "")
		if err != nil {
			return err
		}
		if len(paths) != 1 {
			return ErrExitFalse
		}
		view := ProcessViewFromContext(ctx)
		var target string
		if options.has('f') {
			target, err = canonicalPath(view, paths[0])
		} else if target, err = readLink(view, paths[0]); err != nil && options.has('v') {
			fmt.Fprintf(stderr, "readlink: %s: cannot read link: %s\n", paths[0], err)
		}
		if err != nil {
			return ErrExitFalse
		}
		if !options.has('n') {
			target += "\n"
		}
		_, err = io.WriteString(stdout, target)
		return err
	}}
}

// readLink is the target a link holds, or why there is none, as busybox words it: a file that
// is not a link is `not a symlink`, where strerror would say `Invalid argument`.
func readLink(view ProcessView, operand string) (string, error) {
	native, err := resolveHostPath(view, operand)
	if err != nil {
		return "", err
	}
	target, err := os.Readlink(native)
	if err == nil {
		return target, nil
	}
	if info, statErr := os.Lstat(native); statErr == nil && info.Mode()&os.ModeSymlink == 0 {
		return "", fmt.Errorf("not a symlink")
	}
	return "", fmt.Errorf("%s", causeText(err))
}

// canonicalPath is readlink -f's answer, the path realpath prints, except that the last
// component need not exist, as busybox's xmalloc_realpath_coreutils has it.
func canonicalPath(view ProcessView, operand string) (string, error) {
	resolved, err := ResolveProcessPath(view, operand)
	if err != nil {
		return "", err
	}
	if resolved.Device {
		if info, statErr := statDeviceOperand(view, operand); statErr != nil || info == nil {
			return "", fmt.Errorf("No such file or directory")
		}
		return string(resolved.Canonical), nil
	}
	native, err := realpathAbs(resolved.Native, true)
	if err != nil {
		return "", err
	}
	return canonicalizeGeneratedPath(view, resolved.Canonical, native), nil
}
