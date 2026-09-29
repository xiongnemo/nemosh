package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// chmod is busybox's (coreutils/chmod.c): `chmod [-Rcvf] MODE[,MODE]... FILE...`. MODE is octal
// or symbolic, `u+x,go-w`, and is read against each FILE's own mode; see applyChmodMode. -R
// goes down through a directory after changing it, past the symbolic links it meets there,
// though a FILE named as one is followed. -v reports each FILE, -c only a FILE whose mode
// changes, and -f keeps a failed change quiet.
//
// Only octal was read, so `chmod +x build.sh`, `chmod a-w f` and `chmod -R 755 dir` all ended
// in `invalid mode`.
//
// A MODE may start with a dash, `-w`: the first word that begins with a dash and a letter
// chmod has no option for is the MODE, as busybox reads it. Options may follow the operands,
// as getopt lets them unless POSIXLY_CORRECT is set. A FILE that is not there is named and the
// rest are changed; a MODE that cannot be read ends chmod at the first FILE it is read against.
type chmodRequest struct {
	mode                             string
	umask                            uint32
	recurse, verbose, changes, quiet bool
	stdout, stderr                   io.Writer
	failed                           bool
}

func newChmodApplet() Applet {
	return simpleApplet{name: "chmod", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, stderr io.Writer) error {
		view := ProcessViewFromContext(ctx)
		_, strict := view.LookupEnv("POSIXLY_CORRECT")
		request, files, err := parseChmodArguments(args, !strict)
		if err != nil {
			return err
		}
		request.umask, request.stdout, request.stderr = processFileModeMask(view), stdout, stderr
		for _, file := range files {
			native, err := resolveHostPath(view, file)
			if err != nil {
				request.report(file, err)
				continue
			}
			if err := request.change(ctx, native, file, 0); err != nil {
				return err
			}
		}
		if request.failed {
			return ExitStatus(1)
		}
		return nil
	}}
}

// parseChmodArguments splits the options from the MODE and the FILEs.
func parseChmodArguments(args []string, permute bool) (chmodRequest, []string, error) {
	var request chmodRequest
	mode := -1
	for index, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			break
		}
		if len(arg) > 1 && strings.IndexByte("-Rcvf", arg[1]) < 0 {
			mode = index
			break
		}
	}
	var operands []string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--" {
			operands = append(operands, args[index+1:]...)
			break
		}
		if index == mode || len(arg) < 2 || arg[0] != '-' {
			if operands = append(operands, arg); !permute {
				operands = append(operands, args[index+1:]...)
				break
			}
			continue
		}
		for _, letter := range []byte(arg[1:]) {
			switch letter {
			case 'R':
				request.recurse = true
			case 'c':
				request.changes = true
			case 'v':
				request.verbose = true
			case 'f':
				request.quiet = true
			default:
				return request, nil, invalidOption(letter)
			}
		}
	}
	if len(operands) < 2 {
		return request, nil, missingOperand()
	}
	request.mode = operands[0]
	return request, operands[1:], nil
}

// change applies the MODE to one file, shown by the name it goes by in messages, and with -R
// to what a directory holds. Only a MODE that cannot be read is an error that ends chmod.
func (r *chmodRequest) change(ctx context.Context, native, shown string, depth int) error {
	stat := os.Stat
	if depth > 0 {
		stat = os.Lstat
	}
	info, err := stat(native)
	if err != nil {
		r.report(shown, err)
		return nil
	}
	if depth > 0 && info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	current := currentPermissions(native, info, r.umask)
	mode, ok := applyChmodMode(r.mode, current, r.umask, info.IsDir())
	if !ok {
		return fmt.Errorf("invalid mode '%s'", r.mode)
	}
	if err := applyPermissions(native, mode, info.IsDir()); err != nil {
		if r.failed = true; !r.quiet {
			r.report(shown, err)
		}
		return nil
	}
	if r.verbose || r.changes && current&0o7777 != mode&0o7777 {
		fmt.Fprintf(r.stdout, "mode of '%s' changed to %04o (%s)\n", shown, mode&0o7777, chmodModeLetters(mode))
	}
	if !r.recurse || !info.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(native)
	if err != nil {
		r.report(shown, err)
		return nil
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		inner := shown + "/" + entry.Name()
		if os.IsPathSeparator(shown[len(shown)-1]) {
			inner = shown + entry.Name()
		}
		if err := r.change(ctx, filepath.Join(native, entry.Name()), inner, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// report names a file chmod could not change, and makes its status 1.
func (r *chmodRequest) report(shown string, err error) {
	r.failed = true
	fmt.Fprintf(r.stderr, "chmod: %v\n", operandFailure(shown, err))
}
