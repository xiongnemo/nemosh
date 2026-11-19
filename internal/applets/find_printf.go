package applets

import (
	"fmt"
	"strings"
)

// find -printf FORMAT, GNU's, which busybox's find has not got. FORMAT is read once, as the
// expression is: its escapes, \a \b \c \f \n \r \t \v \\ and \NNN, become what they stand for,
// and each directive, %p and the rest with a width and a precision, a part to fill in for each
// entry; see find_printf_eval.go. An escape or a directive GNU does not know is written as it
// is, with GNU's warning, as GNU's is. Three it knows are refused, before the walk: %F, the file
// system's type, %S, how sparse a file is, and %Z, its SELinux context, none of which this find
// can say.

// findFormatPart is a run of text, a directive, or \c, which ends the entry's output.
type findFormatPart struct {
	text string
	// verb is the directive's letter, 0 for text; when is the k of %Ak, %Ck and %Tk.
	verb, when byte
	// spec is the width and precision as fmt takes them for a string, "%-10.3s"; alternate is
	// the # flag, which %m reads as a leading 0.
	spec      string
	alternate bool
	stop      bool
}

// findPrintfVerbs are the directives an entry fills in.
const findPrintfVerbs = "pfhPHdyYskbmMugUGnilDtac"

func (p *findParser) printfPredicate(operand string) (findNode, error) {
	format, err := p.argument(operand)
	if err != nil {
		return nil, err
	}
	parts, warnings, err := parseFindFormat(format)
	if err != nil {
		return nil, err
	}
	p.hasAction = true
	p.expression.warnings = append(p.expression.warnings, warnings...)
	return findPrintf{parts: parts, umask: processFileModeMask(p.view)}, nil
}

func parseFindFormat(format string) (parts []findFormatPart, warnings []string, err error) {
	var text strings.Builder
	add := func(part findFormatPart) {
		if text.Len() > 0 {
			parts = append(parts, findFormatPart{text: text.String()})
			text.Reset()
		}
		parts = append(parts, part)
	}
	for index := 0; index < len(format); index++ {
		c := format[index]
		switch {
		case c == '\\' && index+1 < len(format):
			index++
			if escape, used, known := findEscape(format[index:]); known {
				text.WriteString(escape)
				index += used - 1
			} else if format[index] == 'c' {
				add(findFormatPart{stop: true})
			} else {
				warnings = append(warnings, fmt.Sprintf("warning: unrecognized escape `\\%c'", format[index]))
				text.WriteString(format[index-1 : index+1])
			}
		case c == '%' && index+1 < len(format):
			flagsEnd := index + 1
			for flagsEnd < len(format) && strings.IndexByte("-+ #0", format[flagsEnd]) >= 0 {
				flagsEnd++
			}
			end := flagsEnd
			for end < len(format) && strings.IndexByte("0123456789.", format[end]) >= 0 {
				end++
			}
			if end >= len(format) {
				text.WriteString(format[index:])
				index = len(format)
				continue
			}
			flags := format[index+1 : flagsEnd]
			part := findFormatPart{verb: format[end], alternate: strings.Contains(flags, "#")}
			// Of the flags only - is kept: every directive is written as a string, and Go's
			// fmt pads a string with zeros for 0, where C's leaves it to spaces.
			part.spec = "%"
			if strings.Contains(flags, "-") {
				part.spec += "-"
			}
			part.spec += format[flagsEnd:end] + "s"
			switch {
			case part.verb == '%':
				text.WriteByte('%')
			case strings.IndexByte("ACT", part.verb) >= 0 && end+1 < len(format):
				end++
				part.when = format[end]
				add(part)
			case strings.IndexByte(findPrintfVerbs, part.verb) >= 0:
				add(part)
			case strings.IndexByte("FSZ", part.verb) >= 0:
				return nil, nil, fmt.Errorf("-printf: %%%c is not something this find can say", part.verb)
			default:
				warnings = append(warnings, fmt.Sprintf("warning: unrecognized format directive `%%%c'", part.verb))
				text.WriteString(format[index : end+1])
			}
			index = end
		default:
			text.WriteByte(c)
		}
	}
	if text.Len() > 0 {
		parts = append(parts, findFormatPart{text: text.String()})
	}
	return parts, warnings, nil
}

// findEscape is the escape at the start of rest, the backslash before it already read: what it
// stands for, how many bytes it took, and whether it is one.
func findEscape(rest string) (string, int, bool) {
	if index := strings.IndexByte(`abfnrtv\`, rest[0]); index >= 0 {
		return string("\a\b\f\n\r\t\v\\"[index]), 1, true
	}
	value, used := 0, 0
	for used < 3 && used < len(rest) && rest[used] >= '0' && rest[used] <= '7' {
		value = value*8 + int(rest[used]-'0')
		used++
	}
	if used == 0 {
		return "", 0, false
	}
	return string(byte(value)), used, true
}
