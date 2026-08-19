package runtime

import (
	"errors"
	"fmt"
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
	// inCondition is set between an unquoted `[[` and its `]]`. See the operator
	// check below for why the lexer has to know.
	inCondition := false
	appendToken := func(token shellToken) error {
		if err := budget.consumeTokens(1); err != nil {
			return err
		}
		if token.kind == tokenWord {
			switch token.value {
			case "[[":
				// Only at the start of a command: `echo [[` is two ordinary words.
				// A word already on the line means this one is an argument.
				if len(tokens) == 0 || tokens[len(tokens)-1].kind != tokenWord {
					inCondition = true
				}
			case "]]":
				inCondition = false
			}
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
			parts = append(parts, wordPart{kind: wordPartEscaped, text: line[index : index+1], quote: quoteFor(inSingle, inDouble)})
			wordPresent = true
			escaped = false
			continue
		}
		if char == '\\' && !inSingle && escapesInsideDoubleQuotes(line, index, inDouble) {
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
		// Before the command substitution branch, because `$((` starts with
		// `$(` and would otherwise be parsed as a subshell inside one.
		if char == '$' && index+2 < len(line) && line[index+1] == '(' && line[index+2] == '(' && !inSingle {
			if end, ok := arithmeticExpansionEnd(line, index+3); ok {
				text := line[index : end+1]
				buffer.WriteString(text)
				parts = append(parts, wordPart{kind: wordPartArithmetic, text: line[index+3 : end-1], quote: quoteFor(inSingle, inDouble)})
				wordPresent = true
				index = end
				continue
			}
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
			// `((expr))` at the start of a command becomes `let expr`, which is the
			// equivalence bash documents. Only at the start: `echo ((` is two
			// ordinary words, and a `(` after a word is not this. See
			// arithmetic_command.go.
			if char == '(' && !wordPresent && (len(tokens) == 0 || tokens[len(tokens)-1].kind != tokenWord) {
				if end := arithmeticCommandEnd(line, index); end > 0 {
					for _, token := range arithmeticCommandTokens(line, index, end) {
						if err := appendToken(token); err != nil {
							return nil, nil, err
						}
					}
					index = end
					continue
				}
			}
			// `@(a|b)` and its four siblings: the whole group is one word, and the
			// `|` inside it is not a pipe. Eighth scan to need this -- without it the
			// group reached the case parser intact and then tokenized into three
			// words, reported as `case: invalid pattern`. See pattern_extended.go.
			if text, ok := extendedGroupText(line, index); ok {
				buffer.WriteString(text)
				appendLiteralPart(&parts, text, quoteUnquoted)
				wordPresent = true
				index += len(text) - 1
				continue
			}
			// `$'...'` -- ANSI-C quoting. Before the array-assignment branch only
			// because both start from an unquoted position; they cannot overlap.
			// The decoded text goes in as a *single-quoted* part, because `$'a b'`
			// is one word in bash: quoting it here is what keeps field splitting
			// and pathname expansion off it. See ansi_quote.go.
			if char == '$' && index+1 < len(line) && line[index+1] == '\'' {
				if text, width, ok := decodeAnsiQuote(line[index:]); ok {
					buffer.WriteString(text)
					appendLiteralPart(&parts, text, quoteSingle)
					wordPresent = true
					index += width - 1
					continue
				}
			}
			// `a=(one two three)` is an array assignment, not a subshell. The `(
			// belongs to the word only when it comes directly after `name=` or
			// `name+=`, which is the test bash applies too -- everywhere else a
			// parenthesis still starts a subshell.
			if char == '(' && looksLikeArrayAssignment(buffer.String()) {
				end, ok := matchingParenthesis(line, index)
				if !ok {
					return nil, nil, fmt.Errorf("%w: missing ) for array assignment", ErrIncompleteScript)
				}
				text := line[index : end+1]
				buffer.WriteString(text)
				appendLiteralPart(&parts, text, quoteUnquoted)
				wordPresent = true
				index = end
				continue
			}
			if char == ' ' || char == '\t' {
				if err := flush(index); err != nil {
					return nil, nil, err
				}
				continue
			}
			// Inside `[[ ]]` none of these are operators. bash makes `[[` a
			// reserved word for exactly this reason: `&&`, `||`, `<` and `>` are
			// part of the conditional's own grammar there, so splitting the line
			// on them would tear the expression apart -- and `[[ a < b ]]`, which
			// is a lexical comparison, would create a file called `b`. Measured
			// before this: it did.
			if kind, width := activeOperator(line[index:]); width > 0 && !inCondition {
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
		// A `${` with no `}` after it used to fall through to the literal
		// branch, so `echo ${x` printed `${x` and exited 0 -- a typo the shell
		// carried out instead of reporting. dash calls it a syntax error and so
		// does this.
		if char == '$' && !inSingle && index+1 < len(line) && line[index+1] == '{' &&
			!strings.ContainsRune(line[index+1:], '}') {
			return nil, nil, errors.New("syntax error: missing '}'")
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
		// The one-byte slice, not string(char): converting a byte to a string
		// goes through rune, so every byte above 0x7F would come out as its own
		// two-byte UTF-8 sequence and a multi-byte character would be shredded.
		// Consecutive literal parts concatenate, so the sequence reassembles.
		appendLiteralPart(&parts, line[index:index+1], quoteFor(inSingle, inDouble))
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
