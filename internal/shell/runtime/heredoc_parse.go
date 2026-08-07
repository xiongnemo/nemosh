package runtime

import (
	"fmt"
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
}

// The source must already have been through normalizeLineEndings.
func collectHeredocs(source string) (string, []pendingHeredoc, error) {
	lines := strings.Split(source, "\n")
	var output strings.Builder
	var records []pendingHeredoc
	for index := 0; index < len(lines); index++ {
		line := lines[index]
		declarations, err := heredocDeclarations(line, index+1, len(records))
		if err != nil {
			return "", nil, err
		}
		output.WriteString(markHeredocOperands(line, declarations))
		if index+1 < len(lines) {
			output.WriteByte('\n')
		}
		for declarationIndex := range declarations {
			declaration := &declarations[declarationIndex]
			var body strings.Builder
			terminated := false
			for index++; index < len(lines); index++ {
				bodyLine := lines[index]
				matched := bodyLine
				if declaration.stripTabs {
					matched = strings.TrimLeft(matched, "\t")
				}
				if matched == declaration.delimiter {
					terminated = true
					break
				}
				if declaration.stripTabs {
					bodyLine = strings.TrimLeft(bodyLine, "\t")
				}
				body.WriteString(bodyLine)
				if index+1 < len(lines) {
					body.WriteByte('\n')
				}
			}
			if !terminated {
				return "", nil, fmt.Errorf("%w: missing heredoc delimiter %q", ErrIncompleteScript, declaration.delimiter)
			}
			declaration.body = body.String()
			records = append(records, *declaration)
		}
	}
	return output.String(), records, nil
}

func heredocDeclarations(line string, lineNumber, startOrder int) ([]pendingHeredoc, error) {
	var records []pendingHeredoc
	quote := byte(0)
	escaped := false
	for index := 0; index < len(line); index++ {
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
		// An arithmetic expansion is stepped over whole, because `$((1<<4))`
		// carries a `<<` that is a shift and not a heredoc.
		if char == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' && quote != '\'' {
			if end, ok := arithmeticExpansionEnd(line, index+3); ok {
				index = end
				continue
			}
		}
		if quote != 0 || char != '<' || index+1 >= len(line) || line[index+1] != '<' {
			if char == '#' && quote == 0 && commentStarts(line, index) {
				break
			}
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
		operandEnd := heredocOperandEnd(line, operandStart)
		if operandEnd == operandStart {
			return nil, fmt.Errorf("%w: %w", ErrIncompleteScript, errMissingRedirectTarget)
		}
		tokens, err := scanShellTokens(line[operandStart:operandEnd])
		if err != nil || len(tokens) != 1 || tokens[0].kind != tokenWord {
			return nil, fmt.Errorf("heredoc delimiter: %w", errMalformedRedirect)
		}
		delimiterWord := parseTypedWord(*tokens[0].parsed)
		delimiter, quoted := quoteRemovedDelimiter(delimiterWord)
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
		})
		index = operandEnd - 1
	}
	return records, nil
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

func heredocOperandEnd(line string, start int) int {
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
		if quote == 0 && (char == ' ' || char == '\t' || char == '|' || char == '&' || char == '<' || char == '>') {
			return index
		}
	}
	return len(line)
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
