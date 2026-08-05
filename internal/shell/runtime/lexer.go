package runtime

import (
	"errors"
	"strings"
)

type tokenKind uint8

const (
	tokenWord tokenKind = iota
	tokenAndIf
	tokenOrIf
	tokenPipe
	tokenRedirect
	tokenBackground
)

type shellToken struct {
	kind            tokenKind
	value           string
	literalDollarAt map[int]struct{}
	parsed          *word
	group           *parsedGroup
}

func scanShellTokens(line string) ([]shellToken, error) {
	return scanShellTokensWithBudget(line, &parseBudget{}, 0)
}

func scanShellTokensWithBudget(line string, budget *parseBudget, depth int) ([]shellToken, error) {
	tokens, _, err := scanShellTokensWithPositions(line, budget, depth)
	return tokens, err
}

func scanShellTokensWithPositions(line string, budget *parseBudget, depth int) ([]shellToken, []int, error) {
	var tokens []shellToken
	var starts []int
	var buffer strings.Builder
	var literalDollarAt map[int]struct{}
	var parts []wordPart
	wordPresent := false
	wordStart := 0
	inSingle := false
	inDouble := false
	escaped := false
	appendToken := func(token shellToken) error {
		if err := budget.consumeTokens(1); err != nil {
			return err
		}
		tokens = append(tokens, token)
		starts = append(starts, wordStart)
		return nil
	}
	flush := func(end int) error {
		if !wordPresent {
			return nil
		}
		value := buffer.String()
		raw := line[wordStart:end]
		typed := &word{
			parts:       append([]wordPart(nil), parts...),
			quotedEmpty: value == "",
			expandTilde: raw == "~" || strings.HasPrefix(raw, "~/"),
		}
		if err := appendToken(shellToken{kind: tokenWord, value: value, literalDollarAt: literalDollarAt, parsed: typed}); err != nil {
			return err
		}
		buffer.Reset()
		parts = nil
		literalDollarAt = nil
		wordPresent = false
		return nil
	}
	for index := 0; index < len(line); index++ {
		char := line[index]
		if !wordPresent {
			wordStart = index
		}
		if escaped {
			if char == '$' {
				if literalDollarAt == nil {
					literalDollarAt = make(map[int]struct{})
				}
				literalDollarAt[buffer.Len()] = struct{}{}
			}
			buffer.WriteByte(char)
			parts = append(parts, wordPart{kind: wordPartEscaped, text: string(char), quote: quoteFor(inSingle, inDouble)})
			wordPresent = true
			escaped = false
			continue
		}
		if char == '\\' && !inSingle {
			wordPresent = true
			escaped = true
			continue
		}
		if char == '\'' && !inDouble {
			wordPresent = true
			inSingle = !inSingle
			continue
		}
		if char == '"' && !inSingle {
			wordPresent = true
			inDouble = !inDouble
			continue
		}
		if char == '$' && index+1 < len(line) && line[index+1] == '(' && !inSingle {
			if end, ok := commandSubstitutionEnd(line, index+2); ok {
				buffer.WriteString(line[index : end+1])
				nested, err := parseScript(line[index+2:end], budget, depth+1)
				part := wordPart{kind: wordPartCommandSubstitution, text: line[index : end+1], quote: quoteFor(inSingle, inDouble)}
				if err != nil {
					return nil, nil, err
				}
				part.script = &nested
				parts = append(parts, part)
				wordPresent = true
				index = end
				continue
			}
		}
		if !inSingle && !inDouble {
			if char == ' ' || char == '\t' {
				if err := flush(index); err != nil {
					return nil, nil, err
				}
				continue
			}
			if kind, width := activeOperator(line[index:]); width > 0 {
				if kind == tokenRedirect && isDigits(line[wordStart:index]) {
					buffer.WriteString(line[index : index+width])
					if err := appendToken(shellToken{kind: tokenRedirect, value: buffer.String()}); err != nil {
						return nil, nil, err
					}
					buffer.Reset()
					parts = nil
					literalDollarAt = nil
					wordPresent = false
					index += width - 1
					continue
				}
				if err := flush(index); err != nil {
					return nil, nil, err
				}
				wordStart = index
				if err := appendToken(shellToken{kind: kind, value: line[index : index+width]}); err != nil {
					return nil, nil, err
				}
				index += width - 1
				continue
			}
		}
		if char == '$' && inSingle {
			if literalDollarAt == nil {
				literalDollarAt = make(map[int]struct{})
			}
			literalDollarAt[buffer.Len()] = struct{}{}
		}
		if char == '$' && !inSingle {
			end := parameterEnd(line, index+1)
			if end > index+1 {
				text := line[index:end]
				buffer.WriteString(text)
				parts = append(parts, wordPart{kind: wordPartParameter, text: text, quote: quoteFor(inSingle, inDouble)})
				wordPresent = true
				index = end - 1
				continue
			}
		}
		buffer.WriteByte(char)
		appendLiteralPart(&parts, string(char), quoteFor(inSingle, inDouble))
		wordPresent = true
	}
	if escaped {
		return nil, nil, errors.New("trailing backslash")
	}
	if inSingle || inDouble {
		return nil, nil, errors.New("unterminated quote")
	}
	if err := flush(len(line)); err != nil {
		return nil, nil, err
	}
	return tokens, starts, nil
}

func quoteFor(inSingle, inDouble bool) quoteContext {
	if inSingle {
		return quoteSingle
	}
	if inDouble {
		return quoteDouble
	}
	return quoteUnquoted
}

func appendLiteralPart(parts *[]wordPart, text string, quote quoteContext) {
	if len(*parts) > 0 {
		last := &(*parts)[len(*parts)-1]
		if last.kind == wordPartLiteral && last.quote == quote {
			last.text += text
			return
		}
	}
	*parts = append(*parts, wordPart{kind: wordPartLiteral, text: text, quote: quote})
}
