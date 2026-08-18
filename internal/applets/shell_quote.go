package applets

import "strings"

// shellQuote spells a string so that a shell reading it back gets the same string.
// It backs `printf %q`.
//
// Measured against bash, which backslash-escapes rather than wrapping in quotes:
//
//	a b     ->  a\ b
//	it's    ->  it\'s
//	plain   ->  plain
//	a$b     ->  a\$b
//	(empty) ->  ''
//
// The empty string is the one case that cannot be done with backslashes, and bash
// spells it `”`.
//
// The safe set is listed rather than the unsafe set, which is the direction that
// fails safely: a character nobody thought about gets escaped instead of getting
// through. `printf %q` exists to make `eval` and generated scripts safe, so the
// cost of an unnecessary backslash is nothing and the cost of a missing one is a
// command that does something else.
func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	var out strings.Builder
	for index := 0; index < len(value); index++ {
		char := value[index]
		if char >= 0x80 {
			// A byte of a multi-byte character. Passed through untouched: it is not
			// special to any shell, and escaping half a rune would corrupt it.
			out.WriteByte(char)
			continue
		}
		switch char {
		case '\n':
			// A newline cannot be backslash-escaped into itself -- `\` then a
			// newline is a line continuation, which would delete it. bash writes
			// $'\n'; this writes the same.
			out.WriteString("$'\\n'")
			continue
		case '\t':
			out.WriteString("$'\\t'")
			continue
		case '\r':
			out.WriteString("$'\\r'")
			continue
		}
		if !isShellSafeByte(char) {
			out.WriteByte('\\')
		}
		out.WriteByte(char)
	}
	return out.String()
}

// isShellSafeByte reports whether a byte means itself to a shell, unquoted.
//
// Letters, digits, and the punctuation that no shell treats specially anywhere: a
// dot in a file name, a dash in an option, an underscore, a slash in a path, a
// colon in a PATH, a plus, a comma, a percent, an at sign.
func isShellSafeByte(char byte) bool {
	switch {
	case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9':
		return true
	}
	switch char {
	case '.', '-', '_', '/', ':', '+', ',', '%', '@':
		return true
	}
	return false
}
