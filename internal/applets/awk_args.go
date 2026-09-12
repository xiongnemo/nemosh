package applets

import (
	"fmt"
	"os"
	"strings"
)

// awk's command line.
//
//	awk [-F fs] [-v var=value]... 'program' [argument...]
//	awk [-F fs] [-v var=value]... -f progfile [-f progfile]... [argument...]
//
// Two things about it are worth stating, because both are easy to get wrong and both were
// measured.
//
// **An operand is a file name or an assignment, and the difference is decided when the
// record loop reaches it, not here.** `awk '{print v, $1}' one v=SET two` prints an empty
// `v` for the records of `one` and `SET` for those of `two`. So the operands are handed to
// the record loop as a list rather than sorted into two piles now.
//
// **A `-v` value and an assignment operand have their escapes processed**, so `-v 'x=a\tb'`
// is three characters and not four. That is why this cannot simply take the text as given.
//
// A value may be joined to its letter or stand apart -- `-F:` and `-F :` are the same, as
// are `-vx=1` and `-v x=1` -- and `--` ends the options.

type awkInvocation struct {
	program string
	// assignments are the `-v` ones, applied before BEGIN runs.
	assignments []string
	// operands are the file names and assignments, in the order they were written. They
	// become ARGV, which a program may rewrite before the record loop reads it.
	operands []string
	// fieldSeparator is `-F`, applied before BEGIN so that a BEGIN block can still
	// override it.
	fieldSeparator string
	hasSeparator   bool
}

func parseAwkArguments(args []string) (*awkInvocation, error) {
	invocation := &awkInvocation{}
	var sources []string
	index := 0
	for ; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			index++
			break
		}
		// A lone `-` is standard input, an operand rather than an option.
		if len(argument) < 2 || argument[0] != '-' {
			break
		}
		letter := argument[1]
		if strings.IndexByte("Fvf", letter) < 0 {
			return nil, fmt.Errorf("awk: invalid option -- %c", letter)
		}
		value := argument[2:]
		if value == "" {
			index++
			if index >= len(args) {
				return nil, fmt.Errorf("-%c needs a value", letter)
			}
			value = args[index]
		}
		if err := invocation.applyOption(letter, value, &sources); err != nil {
			return nil, err
		}
	}
	if len(sources) == 0 {
		if index >= len(args) {
			return nil, missingOperand()
		}
		invocation.program = args[index]
		index++
	} else {
		// Several `-f` files are **one program**, joined by newlines, so a function
		// defined in the first is callable from the second.
		invocation.program = strings.Join(sources, "\n")
	}
	invocation.operands = args[index:]
	return invocation, nil
}

func (v *awkInvocation) applyOption(letter byte, value string, sources *[]string) error {
	switch letter {
	case 'F':
		v.fieldSeparator, v.hasSeparator = awkExpandAssignmentValue(value), true
	case 'v':
		if !strings.Contains(value, "=") {
			return fmt.Errorf("-v takes var=value, not %q", value)
		}
		v.assignments = append(v.assignments, value)
	case 'f':
		text, err := readAwkSource(value)
		if err != nil {
			return err
		}
		*sources = append(*sources, text)
	}
	return nil
}

// readAwkSource reads a program file, through the same UTF-16 decoding every text applet
// uses so that a program saved by a Windows editor is text rather than interleaved NULs.
func readAwkSource(name string) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", cannotOpen(name, err)
	}
	defer file.Close()
	data, err := readAllText(file)
	if err != nil {
		return "", operandFailure(name, err)
	}
	return data, nil
}

// awkOperandAssignment reports whether an operand is `var=value` rather than a file name.
//
// The name has to look like a name: `=x` is a file and so is `1=2`, which matters because a
// file called `2020=report` should be opened rather than assigned.
func awkOperandAssignment(operand string) (string, string, bool) {
	equals := strings.IndexByte(operand, '=')
	if equals <= 0 {
		return "", "", false
	}
	name := operand[:equals]
	for index := 0; index < len(name); index++ {
		character := name[index]
		isLetter := character == '_' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z')
		isDigit := character >= '0' && character <= '9'
		if !isLetter && !(isDigit && index > 0) {
			return "", "", false
		}
	}
	return name, operand[equals+1:], true
}

// awkExpandAssignmentValue processes the escapes a command-line value may contain.
//
// The same table a string literal uses, because that is what both references do: `-v
// 'x=a\tb'` holds a tab, and `-F '\t'` splits on one.
func awkExpandAssignmentValue(text string) string {
	if !strings.Contains(text, `\`) {
		return text
	}
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != '\\' || index+1 >= len(text) {
			out.WriteByte(text[index])
			continue
		}
		replacement, width, err := awkDecodeEscape(text[index+1:], 0)
		if err != nil {
			out.WriteByte(text[index])
			continue
		}
		out.WriteString(replacement)
		index += width
	}
	return out.String()
}
