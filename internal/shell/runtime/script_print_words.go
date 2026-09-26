package runtime

import (
	"strconv"
	"strings"
)

// The words and redirections of a printed script; see scriptPrinter.

func (p *scriptPrinter) redirects(operations []redirectOperation) string {
	var out strings.Builder
	for _, operation := range operations {
		out.WriteByte(' ')
		out.WriteString(p.redirect(operation))
	}
	return out.String()
}

// redirect is one redirection as it would be written, the descriptor left off where it is
// the operator's own.
func (p *scriptPrinter) redirect(operation redirectOperation) string {
	descriptor := func(defaultFD int) string {
		switch {
		case operation.name != "":
			return "{" + operation.name + "}"
		case operation.target == defaultFD:
			return ""
		}
		return strconv.Itoa(operation.target)
	}
	target := printWord(operation.operand)
	switch operation.kind {
	case redirectInput:
		return descriptor(0) + "<" + target
	case redirectOutput:
		if operation.bothStreams {
			return "&>" + target
		}
		return descriptor(1) + ">" + target
	case redirectClobber:
		return descriptor(1) + ">|" + target
	case redirectReadWrite:
		return descriptor(0) + "<>" + target
	case redirectAppend:
		if operation.bothStreams {
			return "&>>" + target
		}
		return descriptor(1) + ">>" + target
	case redirectHeredoc:
		p.pending = append(p.pending, operation)
		delimiter := operation.delimiter
		if !operation.expand {
			delimiter = singleQuoteForReuse(delimiter)
		}
		return descriptor(0) + "<<" + delimiter
	case redirectHereString:
		return descriptor(0) + "<<<" + target
	case redirectDup:
		if operation.duplicate {
			return descriptor(1) + ">&" + target
		}
		moved := ""
		if operation.move {
			moved = "-"
		}
		if operation.target == 0 {
			return "<&" + strconv.Itoa(operation.source) + moved
		}
		return descriptor(1) + ">&" + strconv.Itoa(operation.source) + moved
	case redirectClose:
		if operation.target == 0 {
			return "<&-"
		}
		return descriptor(1) + ">&-"
	}
	return ""
}

// printWord writes a word with the quoting it was parsed with: each run of double-quoted
// parts in one pair of quotes, single-quoted text in single quotes, escapes as escapes.
func printWord(value word) string {
	var out strings.Builder
	for index := 0; index < len(value.parts); {
		part := value.parts[index]
		switch part.quote {
		case quoteSingle:
			out.WriteString(singleQuoteForReuse(part.text))
			index++
		case quoteDouble:
			out.WriteByte('"')
			for ; index < len(value.parts) && value.parts[index].quote == quoteDouble; index++ {
				out.WriteString(printPart(value.parts[index], true))
			}
			out.WriteByte('"')
		default:
			out.WriteString(printPart(part, false))
			index++
		}
	}
	if out.Len() == 0 && value.quotedEmpty {
		return "''"
	}
	return out.String()
}

func printPart(part wordPart, doubleQuoted bool) string {
	switch part.kind {
	case wordPartEscaped:
		return `\` + part.text
	case wordPartArithmetic:
		return "$((" + part.text + "))"
	case wordPartLiteral:
		if doubleQuoted {
			return strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`).Replace(part.text)
		}
	}
	return part.text
}
