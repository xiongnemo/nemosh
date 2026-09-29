package applets

import (
	"fmt"
	"io"
	"path"
	"strings"
)

// `basename -z /a/b` printed `-z`: the flag was taken as the operand, so the
// applet answered a question nobody asked and reported success.
func newBasenameApplet() Applet {
	return simpleApplet{name: "basename", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptionsInOrder(args, "a", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		// -a makes every operand a path to strip, where without it the second
		// operand is a suffix to remove from the first. The two readings are
		// exclusive, which is why the option exists: `basename a b` means one
		// thing and `basename -a a b` means another.
		if options.has('a') {
			for _, operand := range operands {
				fmt.Fprintln(stdout, baseName(operand))
			}
			return nil
		}
		name := baseName(operands[0])
		if len(operands) > 1 {
			name = strings.TrimSuffix(name, operands[1])
		}
		fmt.Fprintln(stdout, name)
		return nil
	}}
}

// baseName is the last component, with a Windows separator treated as one too.
// A path typed on Windows arrives with either, and answering `C:\a\b` with the
// whole string would be answering about a filename nobody has.
func baseName(operand string) string {
	return path.Base(strings.ReplaceAll(operand, "\\", "/"))
}

func newDirnameApplet() Applet {
	return simpleApplet{name: "dirname", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		_, operands, err := parseAppletOptionsInOrder(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		fmt.Fprintln(stdout, dirnameOf(operands[0]))
		return nil
	}}
}

// dirnameOf is POSIX's dirname as busybox-w32 answers it. The trailing separators go first, so
// `/a/b/` is `/a` and `a/` is `.`, where path.Dir answered `/a/b` and `a`. Either slash is a
// separator and is kept as written, `C:\x\y\` giving `C:\x`, and a drive stays with its
// root: `C:/x` is `C:/`, `C:x/y` is `C:x`, and `C:x` is `C:.`.
func dirnameOf(operand string) string {
	drive, rest := "", operand
	if len(rest) >= 2 && rest[1] == ':' && (rest[0]|0x20 >= 'a' && rest[0]|0x20 <= 'z') {
		drive, rest = rest[:2], rest[2:]
	}
	end := len(strings.TrimRight(rest, `/\`))
	if end == 0 {
		if rest == "" {
			return drive + "."
		}
		return drive + rest[:1]
	}
	cut := strings.LastIndexAny(rest[:end], `/\`)
	if cut < 0 {
		return drive + "."
	}
	if parent := strings.TrimRight(rest[:cut], `/\`); parent != "" {
		return drive + parent
	}
	return drive + rest[:1]
}
