package runtime

import (
	"fmt"
	"strings"
)

type syntaxScanner struct {
	lines         []string
	logical       strings.Builder
	quotes        []byte
	substitutions int
	groupClosers  []byte
	syntaxErr     error
	escaped       bool
	continued     bool
}

func logicalLines(source string) ([]string, error) {
	lines, err := partialLogicalLines(source)
	if err != nil {
		return nil, err
	}
	return lines, nil
}

func partialLogicalLines(source string) ([]string, error) {
	physical := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
	if len(physical) > 0 && physical[len(physical)-1] == "" {
		physical = physical[:len(physical)-1]
	}
	scanner := syntaxScanner{}
	for _, line := range physical {
		scanner.beginPhysicalLine()
		scanner.scanLine(line)
		scanner.finishPhysicalLine(line)
	}
	if err := scanner.incompleteError(); err != nil {
		return scanner.lines, err
	}
	scanner.flushLogicalLine()
	return scanner.lines, nil
}

func (scanner *syntaxScanner) beginPhysicalLine() {
	scanner.continued = false
}

func (scanner *syntaxScanner) scanLine(line string) {
	comment := false
	for index := 0; index < len(line); index++ {
		if scanner.syntaxErr != nil {
			return
		}
		char := line[index]
		if comment {
			continue
		}
		if scanner.escaped {
			scanner.logical.WriteByte(char)
			scanner.escaped = false
			continue
		}
		if scanner.quote() == '\'' {
			scanner.logical.WriteByte(char)
			if char == '\'' {
				scanner.popQuote()
			}
			continue
		}
		if char == '\\' {
			if scanner.quote() != '\'' && index == len(line)-1 {
				scanner.continued = true
				continue
			}
			scanner.logical.WriteByte(char)
			scanner.escaped = true
			continue
		}
		if char == '\'' && scanner.quote() == 0 {
			scanner.quotes = append(scanner.quotes, char)
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '"' {
			scanner.toggleDoubleQuote()
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '(' && scanner.quote() != '\'' {
			scanner.substitutions++
			scanner.quotes = append(scanner.quotes, 0)
			scanner.logical.WriteString("$(")
			index++
			continue
		}
		if char == ')' && scanner.substitutions > 0 && scanner.quote() == 0 {
			scanner.substitutions--
			scanner.popQuote()
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && scanner.substitutions == 0 && braceDelimiterAt(line, index, '{') {
			scanner.groupClosers = append(scanner.groupClosers, '}')
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && scanner.substitutions == 0 && char == '(' {
			scanner.groupClosers = append(scanner.groupClosers, ')')
			scanner.logical.WriteByte(char)
			continue
		}
		if scanner.quote() == 0 && scanner.substitutions == 0 && (char == ')' || braceDelimiterAt(line, index, '}')) {
			if len(scanner.groupClosers) == 0 {
				scanner.logical.WriteByte(char)
				continue
			}
			expected := scanner.groupClosers[len(scanner.groupClosers)-1]
			if char != expected {
				scanner.syntaxErr = fmt.Errorf("syntax error: unexpected %c, expected %c", char, expected)
				return
			}
			scanner.groupClosers = scanner.groupClosers[:len(scanner.groupClosers)-1]
			scanner.logical.WriteByte(char)
			continue
		}
		if char == '#' && scanner.quote() == 0 && commentStarts(line, index) {
			comment = true
			continue
		}
		scanner.logical.WriteByte(char)
	}
}

func (scanner *syntaxScanner) finishPhysicalLine(line string) {
	if scanner.continued {
		return
	}
	if scanner.quote() != 0 || scanner.substitutions != 0 || len(scanner.groupClosers) != 0 {
		scanner.logical.WriteByte('\n')
		return
	}
	if hasTrailingSyntaxOperator(line) {
		scanner.logical.WriteByte(' ')
		scanner.continued = true
		return
	}
	scanner.flushLogicalLine()
}

func (scanner *syntaxScanner) flushLogicalLine() {
	if normalized := strings.TrimSpace(scanner.logical.String()); normalized != "" {
		scanner.lines = append(scanner.lines, normalized)
	}
	scanner.logical.Reset()
}

func (scanner *syntaxScanner) incompleteError() error {
	if scanner.syntaxErr != nil {
		return scanner.syntaxErr
	}
	if scanner.continued {
		return fmt.Errorf("%w: trailing line continuation", ErrIncompleteScript)
	}
	if scanner.quote() != 0 {
		return fmt.Errorf("%w: unterminated quote", ErrIncompleteScript)
	}
	if scanner.substitutions != 0 {
		return fmt.Errorf("%w: unterminated command substitution", ErrIncompleteScript)
	}
	if len(scanner.groupClosers) != 0 {
		return fmt.Errorf("%w: missing %c", ErrIncompleteScript, scanner.groupClosers[len(scanner.groupClosers)-1])
	}
	return nil
}

func (scanner *syntaxScanner) quote() byte {
	if len(scanner.quotes) == 0 {
		return 0
	}
	return scanner.quotes[len(scanner.quotes)-1]
}

func (scanner *syntaxScanner) popQuote() {
	scanner.quotes = scanner.quotes[:len(scanner.quotes)-1]
}

func (scanner *syntaxScanner) toggleDoubleQuote() {
	switch scanner.quote() {
	case '"':
		scanner.popQuote()
	case 0:
		scanner.quotes = append(scanner.quotes, '"')
	}
}

func commentStarts(line string, index int) bool {
	return index == 0 || line[index-1] == ' ' || line[index-1] == '\t'
}
