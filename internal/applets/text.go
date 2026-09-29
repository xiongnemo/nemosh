package applets

import (
	"fmt"
	"io"
	"path"
	"strings"
)

// basename is busybox's (coreutils/basename.c): `basename NAME [SUFFIX]`, `basename -a NAME...`
// and `basename -s SUFFIX NAME...`. -a makes every operand a NAME, and -s strips SUFFIX from each
// and implies -a. A SUFFIX is not stripped from a NAME that is nothing else. Without -a or -s a
// third operand is refused, as busybox refuses it.
//
// -s was refused as an invalid option, and `basename a b c` printed a.
//
// `basename -z /a/b` printed `-z`: the flag was taken as the operand, so the applet answered a
// question nobody asked and reported success.
func newBasenameApplet() Applet {
	return simpleApplet{name: "basename", run: func(args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptionsInOrder(args, "a", "s")
		if err != nil {
			return err
		}
		if len(operands) == 0 {
			return missingOperand()
		}
		suffix, all := options.value('s'), options.has('a') || options.has('s')
		if !all {
			if len(operands) > 2 {
				return fmt.Errorf("extra operand '%s'", operands[2])
			}
			if len(operands) == 2 {
				suffix = operands[1]
			}
			operands = operands[:1]
		}
		for _, operand := range operands {
			name := baseName(operand)
			if len(name) > len(suffix) {
				name = strings.TrimSuffix(name, suffix)
			}
			fmt.Fprintln(stdout, name)
		}
		return nil
	}}
}

// baseName is the last component, with a Windows separator treated as one too.
// A path typed on Windows arrives with either, and answering `C:\a\b` with the
// whole string would be answering about a filename nobody has.
func baseName(operand string) string {
	// An empty NAME has no last component to give, and busybox and GNU both print an empty line
	// for it, where path.Base answers ".".
	if operand == "" {
		return ""
	}
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
		// One NAME, as busybox's single_argv takes: a second is refused rather than ignored.
		if len(operands) > 1 {
			return fmt.Errorf("extra operand '%s'", operands[1])
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
