package runtime

import (
	"fmt"
	"strings"
)

// What keeps a logical line open past the end of a physical one, beyond a quote: a
// substitution, a group, and a `[[` conditional.

// substitutionParenthesis follows an unquoted parenthesis inside a `$(`. One of the body's
// own opens a level and closes it -- a subshell's, a function's, an array's, a `<(`'s -- and a
// case pattern's opens and closes nothing, as commandSubstitutionEnd reads them. Every `)`
// was taken for the substitution's, so a subshell or a pattern ended it early: `{ x=$( (a) );
// }` was `unexpected ), expected }`, and `x=$(case x in` then `x) echo hit;;` then `esac)`
// an unterminated command substitution, where busybox-w32 and bash run both.
func (scanner *syntaxScanner) substitutionParenthesis(char byte) {
	open := &scanner.substitutions[len(scanner.substitutions)-1]
	body := scanner.logical.String()[open.body:]
	open.words.extend(body)
	switch {
	case open.words.openAt(len(body)):
	case char == '(':
		open.depth++
	case open.depth > 0:
		open.depth--
	default:
		scanner.substitutions = scanner.substitutions[:len(scanner.substitutions)-1]
		scanner.popQuote()
	}
}

// followCondition notes the `[[` that opens a conditional and the `]]` that closes it, outside
// quotes and substitutions: a conditional goes on past the end of its line, as bash's does, so
// `[[ foo == foo` and `&& bar == bar ]]` on the next line are one command, and `[[` alone on
// its line begins one. It went on only after `&&`, `||` or `(`, as any command does, and a
// newline anywhere else ended it unclosed. The `[[` counts only where a command begins, so
// `echo [[` is a word, and the `]]` only as a word of its own.
func (scanner *syntaxScanner) followCondition(line string, index int) {
	if scanner.quote() != 0 || len(scanner.substitutions) != 0 {
		return
	}
	before := scanner.logical.String()
	switch {
	// Asked of the line so far and the rest of this one as two strings: joined, every `[[` in
	// a function's body copied the body up to it.
	case !scanner.condition && strings.HasPrefix(line[index:], "[["):
		scanner.condition = opensConditionAfter(before, line[index:]) && scanner.commandBegins(before)
	case scanner.condition && strings.HasPrefix(line[index:], "]]") && endsConditionWord(line, index+2):
		scanner.condition = before != "" && strings.IndexByte(" \t\n)", before[len(before)-1]) < 0
	}
}

// commandBegins reports whether a command begins at the end of the logical line so far, before:
// at its start, after a separator, an opening bracket or `!`, after a reserved word that a
// command follows, or after the `)` of a function's `()` or of a case pattern -- which is the
// scanner's own question of the line, kept as it goes (syntax_scan_case.go).
func (scanner *syntaxScanner) commandBegins(before string) bool {
	previous, found := previousNonBlank(before, len(before))
	if previous == ')' {
		return strings.HasSuffix(strings.TrimRight(before, " \t"), "()") || scanner.caseOpen()
	}
	return !found || strings.IndexByte(";&|({!\n", previous) >= 0 || afterCommandIntroducer(before, len(before))
}

func (scanner *syntaxScanner) incompleteError() error {
	if scanner.syntaxErr != nil {
		return scanner.syntaxErr
	}
	if scanner.continued {
		return fmt.Errorf("%w: trailing line continuation", ErrIncompleteScript)
	}
	if scanner.inBraceParameter() {
		return fmt.Errorf("%w: missing '}'", ErrIncompleteScript)
	}
	if scanner.quote() != 0 {
		return fmt.Errorf("%w: unterminated quote", ErrIncompleteScript)
	}
	if len(scanner.substitutions) != 0 {
		return fmt.Errorf("%w: unterminated command substitution", ErrIncompleteScript)
	}
	if len(scanner.groupClosers) != 0 {
		return fmt.Errorf("%w: missing %c", ErrIncompleteScript, scanner.groupClosers[len(scanner.groupClosers)-1])
	}
	if scanner.condition {
		return fmt.Errorf("%w: missing ]]", ErrIncompleteScript)
	}
	return nil
}
