package runtime

import "strings"

// reservedWordBeforeParen is whether the `(` at index follows, with no blank between, a
// reserved word that a command comes after: `if(true)`, `while((i < 2))`, `do(echo x)`. The `(`
// is an operator, so the word ends before it, as both references read it, and bash runs
// `if((1))` as its arithmetic command. The two were one word, `if(true)`, that was no `if`, and
// its `then` stood alone: "unexpected then". Only where a command begins, so an argument
// such as `echo if(x)` is left as it is.
func reservedWordBeforeParen(line string, index int) bool {
	start := index
	for start > 0 && isNameByte(line[start-1]) {
		start--
	}
	switch line[start:index] {
	case "if", "elif", "while", "until", "then", "else", "do":
	default:
		return false
	}
	back := previousNonBlankIndex(line, start)
	return back < 0 || strings.IndexByte(";&|\n({", line[back]) >= 0 || afterCommandIntroducer(line, start)
}
