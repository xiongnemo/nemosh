package runtime

import (
	"fmt"
	"slices"
	"strings"
)

type pendingHeredoc struct {
	delimiterWord word
	delimiter     string
	expand        bool
	stripTabs     bool
	body          string
	line          int
	order         int
	marker        string
	operandStart  int
	operandEnd    int
	// depth is how many substitutions its `<<` was inside; see heredocScan.
	depth int
}

// The source must already have been through normalizeLineEndings. The third answer is,
// for each line left in the output, the index of the source line it was, so $LINENO can
// count past the bodies taken out (line_numbers.go).
//
// atEnd is whether the source is all there is, a script's or eval's rather than what a
// prompt has read so far. There a body the end reaches before its delimiter ends with it,
// as both references end it, and the fourth answer says so in bash's warning; at a prompt
// the body goes on at the next line read. It was refused, the whole script with it.
//
// A body begins after the line its command is on has ended, and a line left inside a quote
// or ending in a backslash goes on into the next: `cat <<EOF \` then `; echo two`, or `cat
// <<EOF; echo "two` then `three"`, as busybox and bash both read them, at the next newline
// token. The body was taken from the very next line, which refused both as incomplete, and
// each line was scanned as if no quote were open, so one left open hid the next line's `<<`.
func collectHeredocs(source string, atEnd bool) (string, []pendingHeredoc, []int, []heredocAtEnd, error) {
	lines := strings.Split(source, "\n")
	var output strings.Builder
	var records, pending []pendingHeredoc
	var origins []int
	var ended []heredocAtEnd
	var scan heredocScan
	for index := 0; index < len(lines); index++ {
		start := index
		line, declarations, next, err := continuedHeredocDeclarations(lines, &index, len(records)+len(pending), scan)
		if err != nil {
			return "", nil, nil, nil, err
		}
		origins = append(origins, start)
		output.WriteString(markHeredocOperands(line, declarations))
		if index+1 < len(lines) {
			output.WriteByte('\n')
		}
		pending, scan = append(pending, declarations...), next
		if scan.inString() || scan.joined {
			continue
		}
		var waiting []pendingHeredoc
		for declarationIndex := range pending {
			declaration := &pending[declarationIndex]
			if declaration.depth < len(scan.quotes) {
				waiting = append(waiting, *declaration)
				continue
			}
			body, terminated := heredocBody(lines, &index, *declaration)
			if !terminated {
				if !atEnd {
					return "", nil, nil, nil, fmt.Errorf("%w: missing heredoc delimiter %q", ErrIncompleteScript, declaration.delimiter)
				}
				body, ended = endedByEOF(body, *declaration, lines, ended)
			}
			declaration.body = body
			records = append(records, *declaration)
		}
		pending = waiting
	}
	if len(pending) > 0 && !atEnd {
		return "", nil, nil, nil, fmt.Errorf("%w: missing heredoc delimiter %q", ErrIncompleteScript, pending[0].delimiter)
	}
	for _, declaration := range pending {
		declaration.body, ended = endedByEOF("", declaration, lines, ended)
		records = append(records, declaration)
	}
	return output.String(), records, origins, ended, nil
}

// heredocScan is where a line leaves the scan for heredocs: the quoting it ends inside,
// innermost last, and whether it ends in a backslash that joins the next line on. A `(` there
// is a `$(`, or a parenthesis within one, whose text is quoted afresh: in `x="$(cat <<EOF` the
// `<<` is a heredoc's, and its body begins on the next line. One heredoc outside the `$(` takes
// its body after the line the `$(` closes on. Both references read them so. One quote was all
// the scan kept, so the `"` hid the `<<`, and `cat <<EOF; x=$(` took its body from inside
// the substitution; each script was refused.
type heredocScan struct {
	quotes []byte
	joined bool
}

func (scan heredocScan) quote() byte {
	if len(scan.quotes) == 0 {
		return 0
	}
	return scan.quotes[len(scan.quotes)-1]
}

// inString is whether the line ended inside a quoted string, whose newline is the string's.
func (scan heredocScan) inString() bool {
	return scan.quote() == '\'' || scan.quote() == '"'
}

// heredocDeclarations finds the heredocs a line declares, the line beginning inside the
// quoting the one before left open, and says where this one leaves it.
func heredocDeclarations(line string, lineNumber, startOrder int, scan heredocScan) ([]pendingHeredoc, heredocScan, error) {
	var records []pendingHeredoc
	scan.quotes = slices.Clone(scan.quotes)
	escaped := false
	for index := 0; index < len(line); index++ {
		char, quote := line[index], scan.quote()
		unquoted := quote == 0 || quote == '('
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if char == quote && (char == '\'' || char == '"') {
			scan.quotes = scan.quotes[:len(scan.quotes)-1]
			continue
		}
		if unquoted && (char == '\'' || char == '"') {
			scan.quotes = append(scan.quotes, char)
			continue
		}
		if end := ansiQuoteClose(line, index); unquoted && end >= 0 {
			index = end
			continue
		}
		// An arithmetic expansion is stepped over whole, because `$((1<<4))`
		// carries a `<<` that is a shift and not a heredoc.
		if char == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' && quote != '\'' {
			if end, ok := arithmeticExpansionEnd(line, index+3); ok {
				index = end
				continue
			}
		}
		// So is an arithmetic command, `(( x << 2 ))`, which went looking for a heredoc
		// delimited by 2.
		if char == '(' && unquoted && index+1 < len(line) && line[index+1] == '(' {
			if end, ok := arithmeticExpansionEnd(line, index+2); ok {
				index = end
				continue
			}
		}
		switch {
		case quote != '\'' && char == '$' && index+1 < len(line) && line[index+1] == '(':
			scan.quotes = append(scan.quotes, '(')
			index++
			continue
		case quote == '(' && char == '(':
			scan.quotes = append(scan.quotes, '(')
			continue
		case quote == '(' && char == ')':
			scan.quotes = scan.quotes[:len(scan.quotes)-1]
			continue
		}
		if !unquoted || char != '<' || index+1 >= len(line) || line[index+1] != '<' {
			// A `<` the line's backslash goes on from may be a `<<`'s first half; see
			// heredoc_continue.go.
			if unquoted && char == '<' && index+2 == len(line) && line[index+1] == '\\' {
				return nil, scan, errHeredocContinued
			}
			if char == '#' && unquoted && commentStarts(line, index) {
				break
			}
			continue
		}
		// `<<<` is a here-string, not a heredoc: its body is on the same line and
		// there is no delimiter to go looking for on the following ones. Without
		// this the scanner read the third `<` as the start of a delimiter, found
		// nothing usable, and reported the whole script incomplete.
		if index+2 < len(line) && line[index+2] == '<' {
			index += 2
			continue
		}
		stripTabs := index+2 < len(line) && line[index+2] == '-'
		operandStart := index + 2
		if stripTabs {
			operandStart++
		}
		for operandStart < len(line) && (line[operandStart] == ' ' || line[operandStart] == '\t') {
			operandStart++
		}
		operandEnd, continued := heredocOperandEnd(line, operandStart)
		if continued {
			return nil, scan, errHeredocContinued
		}
		if operandEnd == operandStart {
			return nil, scan, fmt.Errorf("syntax error: %w", errMissingRedirectTarget)
		}
		tokens, err := scanShellTokens(line[operandStart:operandEnd])
		if err != nil || len(tokens) != 1 || tokens[0].kind != tokenWord {
			return nil, scan, fmt.Errorf("heredoc delimiter: %w", errMalformedRedirect)
		}
		delimiterWord := parseTypedWord(*tokens[0].parsed)
		delimiter, quoted := quoteRemovedDelimiter(delimiterWord)
		// A backquote in the word is as a quote there, and the body is not expanded, as
		// busybox reads a delimiter (readtoken1 under CHKEOFMARK): `cat <<EO`true`F` leaves a
		// $x and a backquoted command in its body as written. bash expands them.
		quoted = quoted || strings.Contains(line[operandStart:operandEnd], "`")
		records = append(records, pendingHeredoc{
			delimiterWord: delimiterWord,
			delimiter:     delimiter,
			expand:        !quoted,
			stripTabs:     stripTabs,
			line:          lineNumber,
			order:         startOrder + len(records),
			marker:        fmt.Sprintf("__nemosh_heredoc_%d__", startOrder+len(records)),
			operandStart:  operandStart,
			operandEnd:    operandEnd,
			depth:         len(scan.quotes),
		})
		index = operandEnd - 1
	}
	scan.joined = escaped
	return records, scan, nil
}

func markHeredocOperands(line string, records []pendingHeredoc) string {
	if len(records) == 0 {
		return line
	}
	var marked strings.Builder
	start := 0
	for _, record := range records {
		marked.WriteString(line[start:record.operandStart])
		marked.WriteString(record.marker)
		start = record.operandEnd
	}
	marked.WriteString(line[start:])
	return marked.String()
}

// heredocOperandEnd is where a heredoc's word ends, and whether it ends the line in a backslash
// that the next line continues it past.
func heredocOperandEnd(line string, start int) (int, bool) {
	quote := byte(0)
	escaped := false
	for index := start; index < len(line); index++ {
		char := line[index]
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if char == '\'' && quote != '"' || char == '"' && quote != '\'' {
			if quote == char {
				quote = 0
			} else if quote == 0 {
				quote = char
			}
			continue
		}
		if end := ansiQuoteClose(line, index); quote == 0 && end >= 0 {
			index = end
			continue
		}
		// Where a word ends: at a blank or an operator character. `;`, `(` and `)` among
		// them, so `cat <<EOF;` is delimited by EOF; the `;` was taken into the delimiter,
		// no line matched, and the script was refused as incomplete.
		if quote == 0 && strings.IndexByte(" \t|&<>;()", char) >= 0 {
			return index, false
		}
	}
	return len(line), escaped
}

func quoteRemovedDelimiter(delimiter word) (string, bool) {
	var value strings.Builder
	quoted := delimiter.quotedEmpty
	for _, part := range delimiter.parts {
		value.WriteString(part.text)
		if part.quote != quoteUnquoted || part.kind == wordPartEscaped {
			quoted = true
		}
	}
	return value.String(), quoted
}
