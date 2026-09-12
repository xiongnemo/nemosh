package applets

// awk's tokens.
//
// Split from the scanner so the tables stay readable and the scanner stays under the
// line ceiling; the interesting decisions are all in awk_lex.go.

type awkTokenKind uint8

const (
	awkTokenEOF awkTokenKind = iota
	// awkTokenNewline is a token rather than whitespace, because in awk a newline
	// **terminates a statement** everywhere except after a handful of tokens. The
	// scanner decides which, so the parser never sees a newline that does not matter.
	awkTokenNewline
	awkTokenName
	awkTokenNumber
	awkTokenString
	// awkTokenRegex is `/re/`, which the scanner can only recognise with context --
	// see awkSlashIsDivision.
	awkTokenRegex
	// awkTokenFuncName is a name immediately followed by `(`, with no space. awk
	// distinguishes it because `f (x)` is a concatenation of `f` and `x` while `f(x)`
	// is a call, and the difference is the space.
	awkTokenFuncName
	awkTokenBuiltin
	awkTokenKeyword
	awkTokenOperator
)

type awkToken struct {
	kind awkTokenKind
	// text is the token as written for a name, keyword or operator, and the *decoded*
	// contents for a string or regex -- so `"a\tb"` arrives with a real tab.
	text string
	// number is set for awkTokenNumber.
	number float64
	line   int
}

// awkKeywords are the words the grammar reserves.
//
// `func` is gawk's abbreviation for `function` and busybox accepts it too, so it is here.
var awkKeywords = map[string]bool{
	"BEGIN": true, "END": true, "function": true, "func": true,
	"if": true, "else": true, "while": true, "for": true, "do": true,
	"break": true, "continue": true, "next": true, "nextfile": true,
	"exit": true, "return": true, "delete": true, "in": true,
	"getline": true, "print": true, "printf": true,
}

// awkBuiltins are the functions the language provides.
//
// They are a separate kind from a keyword because most may be called without
// parentheses in the one case awk allows -- `length` alone means `length($0)`, which
// both references confirm prints 0 on empty input.
var awkBuiltins = map[string]bool{
	"length": true, "substr": true, "index": true, "split": true,
	"sub": true, "gsub": true, "match": true, "sprintf": true,
	"sin": true, "cos": true, "atan2": true, "exp": true, "log": true,
	"sqrt": true, "int": true, "rand": true, "srand": true,
	"tolower": true, "toupper": true, "system": true, "close": true,
	"fflush": true,
}

// awkOperators are the punctuation tokens, longest first so that `>=` is not read as `>`
// and then `=`, and `!~` is not `!` and then `~`.
//
// Ordering is the whole correctness argument here, which is why it is a slice rather
// than the map the keywords use.
var awkOperators = []string{
	"**=", ">>",
	"+=", "-=", "*=", "/=", "%=", "^=", "**",
	"==", "!=", "<=", ">=", "&&", "||", "++", "--", "!~",
	"+", "-", "*", "/", "%", "^", "=", "<", ">", "!", "~",
	"?", ":", ";", ",", "{", "}", "(", ")", "[", "]", "$", "|",
}

// awkContinuesLine reports whether a newline directly after this token is ignored rather
// than ending the statement.
//
// Measured against both references, which agree on every entry: a newline continues after
// `{`, `&&`, `||`, `,`, `do`, `else`, `?` and `:`. It does **not** continue after `(` --
// both refuse that -- and the one place they disagree is after `=`, where busybox accepts
// the continuation and gawk refuses it. POSIX and gawk are followed there, because
// accepting it would mean running a program that is a syntax error in every other awk,
// which is the quiet kind of divergence this project avoids. Recorded in the support
// matrix.
func awkContinuesLine(token awkToken) bool {
	switch token.kind {
	case awkTokenOperator:
		switch token.text {
		case "{", "&&", "||", ",", "?", ":":
			return true
		}
	case awkTokenKeyword:
		switch token.text {
		case "do", "else":
			return true
		}
	case awkTokenNewline, awkTokenEOF:
		// Runs of blank lines collapse rather than producing a terminator each.
		return true
	}
	return false
}

// awkSlashIsDivision decides awk's hardest lexing question from the previous token.
//
// `/` begins a regular expression *unless* what came before it could end an expression,
// in which case it is division. The classic ambiguity settles it: both references answer
// 1 for `a=4; b=2; print a /b/ 2`, which is `4/2/2` -- so after a **name** a slash
// divides. Measured the same way for a number (`$1/2` is 4), for `)` (`length()/2` is 2)
// and for `(4)/2`.
//
// A postfix `++`/`--` ends an expression too; a prefix one does not, but the scanner
// cannot tell them apart and neither can any other awk. `a++ /2/ 3` is vanishingly rare
// and both references make the same choice this does.
func awkSlashIsDivision(previous awkToken) bool {
	switch previous.kind {
	case awkTokenName, awkTokenNumber, awkTokenString:
		return true
	case awkTokenBuiltin:
		// `length` may stand alone, so it can end an expression.
		return true
	case awkTokenOperator:
		switch previous.text {
		case ")", "]", "++", "--":
			return true
		}
	}
	// `$` is deliberately absent: it is a prefix, so what follows it is an operand and
	// `$/re/` is the field numbered by a match rather than a division. Nothing can be
	// divided before there is something to divide.
	return false
}
