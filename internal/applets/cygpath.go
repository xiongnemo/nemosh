package applets

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/xiongnemo/nemosh/internal/pathmodel"
)

// cygpath is Git for Windows' cygpath -- MSYS2's -- over this shell's path model, for a script
// written for Git's bash that converts a path before handing it to a Windows program: zoxide's
// bash init runs `cygpath -w "$(pwd -L)"` before every prompt. -u is the Unix spelling, /c/Users,
// and the default; -w the Windows one with backslashes; -m with forward slashes, as winpath
// prints it; -a makes a relative path absolute, which is otherwise only respelled; -p converts a
// list, a Unix one split at : for -w and -m and a Windows one at ; for -u. busybox has none.
// Git's other options -- -l, -s, the special folders -- are refused by name.
func newCygpathApplet() Applet {
	return simpleApplet{name: "cygpath", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletLongOptions(ctx, args, "unix=u windows=w mixed=m absolute=a path=p", "uwmap", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return fmt.Errorf("a path is needed: cygpath [-u|-w|-m] [-a] [-p] PATH...")
		}
		form := byte('u')
		for _, letter := range []byte("wm") {
			if options.has(letter) {
				form = letter
			}
		}
		view := ProcessViewFromContext(ctx)
		model, err := cwdPathModel(view.WorkingDirectory())
		if err != nil {
			return err
		}
		status := error(nil)
		for _, operand := range operands {
			converted, err := cygpathList(view, model, operand, form, options.has('a'), options.has('p'))
			if err != nil {
				fmt.Fprintf(stderr, "cygpath: %s: %v\n", operand, err)
				status = ErrExitFalse
				continue
			}
			fmt.Fprintln(stdout, converted)
		}
		return status
	}}
}

// cygpathList converts text, or each path of the list it is with -p.
func cygpathList(view ProcessView, model pathmodel.Model, text string, form byte, absolute, list bool) (string, error) {
	if !list {
		return cygpathOne(view, model, text, form, absolute)
	}
	in, out := ":", ";"
	if form == 'u' {
		in, out = ";", ":"
	}
	paths := strings.Split(text, in)
	for index, path := range paths {
		converted, err := cygpathOne(view, model, path, form, absolute)
		if err != nil {
			return "", err
		}
		paths[index] = converted
	}
	return strings.Join(paths, out), nil
}

// cygpathOne converts one path: a relative one respelled, unless absolute asks it resolved, and
// an absolute one as winpath or posixpath answers it.
func cygpathOne(view ProcessView, model pathmodel.Model, path string, form byte, absolute bool) (string, error) {
	if path == "" {
		return "", nil
	}
	if !absolute && !cygpathAbsolute(path) {
		if form == 'w' {
			return strings.ReplaceAll(path, "/", `\`), nil
		}
		return strings.ReplaceAll(path, `\`, "/"), nil
	}
	converted, err := convertedPath(view, model, path, form != 'u', func(model pathmodel.Model, arg string) (string, error) {
		resolved, err := model.Resolve(arg)
		if err != nil || form == 'u' {
			return string(resolved), err
		}
		return pathmodel.WindowsPath(resolved)
	})
	if err != nil || form != 'w' {
		return converted, err
	}
	return strings.ReplaceAll(converted, "/", `\`), nil
}

// cygpathAbsolute is a path that names its place whatever the working directory: one from a
// root, a UNC one, or one from a drive.
func cygpathAbsolute(path string) bool {
	return strings.HasPrefix(path, "/") || strings.HasPrefix(path, `\`) ||
		len(path) >= 2 && path[1] == ':' && (path[0]|0x20 >= 'a' && path[0]|0x20 <= 'z')
}
