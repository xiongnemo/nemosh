package applets

import "regexp"

// Python, shell, Haskell and Prolog -- the four whose string and comment rules are each
// unlike the C family's, and two of which have a trap worth naming.

func pythonSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "python",
		extensions: []string{".py", ".pyi", ".pyw"},
		regions: []highlightRegion{
			// The triple-quoted forms first, and that order is the whole point: `"""`
			// starts with `"`, so a plain-string rule tried first would match one
			// character and leave the other two to be read as an empty string. Regions
			// are tried in order, so the longer delimiter has to come first.
			{start: regexp.MustCompile(`^"""`), end: regexp.MustCompile(`^"""`), group: groupString},
			{start: regexp.MustCompile(`^'''`), end: regexp.MustCompile(`^'''`), group: groupString},
			quoted(`"`, true),
			quoted(`'`, true),
		},
		patterns: []highlightPattern{
			lineComment("#", groupComment),
			// A decorator owns its name, which is what makes @property read as one
			// thing rather than as an operator and an identifier.
			expr(`^@[A-Za-z_][A-Za-z0-9_.]*`, groupKeyword, false),
			words(groupKeyword,
				"and", "as", "assert", "async", "await", "break", "class", "continue",
				"def", "del", "elif", "else", "except", "finally", "for", "from", "global",
				"if", "import", "in", "is", "lambda", "match", "nonlocal", "not", "or",
				"pass", "raise", "return", "try", "while", "with", "yield"),
			words(groupType,
				"None", "True", "False", "self", "cls", "bool", "bytes", "bytearray",
				"complex", "dict", "float", "frozenset", "int", "list", "object", "set",
				"str", "tuple", "type", "len", "print", "range", "enumerate", "zip",
				"open", "isinstance", "super", "Exception", "ValueError", "TypeError"),
			numbers(),
			symbols("**=", "//=", "->", ":=", "==", "!=", "<=", ">=", "**", "//",
				"+=", "-=", "*=", "/=", "%=", "+", "-", "*", "/", "%", "=", "<", ">",
				"(", ")", "{", "}", "[", "]", ",", ";", ":", "."),
		},
	}
}

func shellSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "shell",
		extensions: []string{".sh", ".bash", ".zsh", ".ksh", ".profile", ".bashrc", ".zshrc"},
		filenames:  []string{".profile", ".bashrc", ".bash_profile", ".zshrc", "profile"},
		regions: []highlightRegion{
			quoted(`"`, true),
			// A single-quoted shell string has **no escapes at all**: a backslash
			// inside one is a backslash, and only the next quote ends it. So no skip,
			// which is the opposite of the double-quoted form above and the thing a
			// C-shaped table would get wrong.
			quoted(`'`, false),
		},
		patterns: []highlightPattern{
			lineComment("#", groupComment),
			// A variable, in all three spellings. Before the operator list, or the `$`
			// is an operator and the name an identifier.
			expr(`^\$(?:\{[^}]*\}|\([^)]*\)|[A-Za-z_][A-Za-z0-9_]*|[0-9@*#?$!-])`, groupType, false),
			words(groupKeyword,
				"if", "then", "elif", "else", "fi", "for", "while", "until", "do", "done",
				"case", "esac", "in", "function", "select", "time", "coproc",
				"return", "break", "continue", "exit", "local", "export", "readonly",
				"declare", "typeset", "unset", "shift", "eval", "exec", "trap", "set"),
			words(groupType,
				"echo", "printf", "read", "test", "cd", "pwd", "source", "alias",
				"command", "builtin", "type", "hash", "umask", "wait", "jobs", "kill"),
			numbers(),
			symbols("&&", "||", ">>", "<<", ";;", "|&", ">&", "<&",
				"|", "&", ";", "<", ">", "=", "(", ")", "{", "}", "[", "]", "!"),
		},
	}
}

func haskellSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "haskell",
		extensions: []string{".hs", ".lhs"},
		regions: []highlightRegion{
			// The pragma before the comment, because `{-#` starts with `{-` and would
			// otherwise be read as an ordinary block comment. It is not one -- it is
			// instructions to the compiler -- so it gets the keyword colour.
			{
				start: regexp.MustCompile(`^\{-#`),
				end:   regexp.MustCompile(`^#-\}`),
				group: groupKeyword,
			},
			// **Nested**, which is the language's actual rule and the reason the engine
			// has the flag at all: `{- {- -} -}` is one comment and the inner closer
			// does not end the outer.
			blockComment("{-", "-}", true),
			quoted(`"`, true),
		},
		patterns: []highlightPattern{
			lineComment("--", groupComment),
			// A character literal as a bounded pattern and *not* a region, which
			// matters more here than anywhere else: a prime is a legal identifier
			// character in Haskell, so `f x' = x'` would open a region on the
			// apostrophe and swallow the rest of the file.
			expr(`^'(?:\\.|[^'\\])'`, groupString, false),
			words(groupKeyword,
				"case", "class", "data", "default", "deriving", "do", "else", "foreign",
				"if", "import", "in", "infix", "infixl", "infixr", "instance", "let",
				"module", "newtype", "of", "then", "type", "where", "forall", "mdo", "rec"),
			words(groupType,
				"Bool", "Char", "Double", "Either", "Float", "IO", "Int", "Integer",
				"Maybe", "Ordering", "String", "Word", "True", "False", "Just", "Nothing",
				"Left", "Right", "LT", "EQ", "GT", "Functor", "Applicative", "Monad",
				"Monoid", "Semigroup", "Show", "Eq", "Ord", "Num", "Foldable", "Traversable"),
			// A constructor or a type is capitalised, which is a rule the language
			// enforces -- so it is a highlightable fact and not a convention.
			expr(`^[A-Z][A-Za-z0-9_']*`, groupType, true),
			numbers(),
			symbols("<-", "->", "=>", "::", "..", "<>", "<$>", "<*>", ">>=", ">=>", "$!",
				"++", "&&", "||", "==", "/=", "<=", ">=", ">>", "<<", "|", "@", "~",
				"+", "-", "*", "/", "=", "<", ">", "$", ".", "!", "\\",
				"(", ")", "{", "}", "[", "]", ",", ";", ":"),
		},
	}
}

func prologSyntax() *highlightSyntax {
	return &highlightSyntax{
		name: "prolog",
		// `.pl` is Perl's too and Perl is not in this set; a `.pl` here is Prolog.
		extensions: []string{".pl", ".pro", ".plt", ".prolog", ".ecl"},
		regions: []highlightRegion{
			blockComment("/*", "*/", false),
			quoted(`"`, true),
			// A quoted atom. Safe as a region *because* the character-code pattern
			// below is matched first -- see the comment there.
			quoted(`'`, true),
		},
		patterns: []highlightPattern{
			lineComment("%", groupComment),
			// **The character-code notation, and it must come before everything.**
			// `0'a` is the integer for `a`, and `0'\n` for a newline. The quote in it
			// is not a delimiter, so if the quoted-atom region were reached first it
			// would open on that quote and swallow the rest of the line. Patterns run
			// after regions in general -- but the scan is left to right, and this
			// matches at the `0`, one character before the region could start. That
			// ordering is what makes it work, and it is why the sample file for this
			// language has `X = 0'a, Y = 0'\n` in it.
			expr(`^0'(?:\\.|.)`, groupNumber, true),
			words(groupKeyword,
				"module", "use_module", "dynamic", "discontiguous", "multifile",
				"initialization", "is", "mod", "rem", "div", "xor", "rdiv",
				"assert", "asserta", "assertz", "retract", "findall", "bagof", "setof",
				"forall", "between", "succ_or_zero", "catch", "throw", "true", "fail",
				"false", "not", "once", "ignore", "call", "atom", "number", "var",
				"nonvar", "functor", "arg", "copy_term", "write", "writeln", "format",
				"nl", "read", "halt", "consult", "op"),
			// A variable is capitalised or begins with an underscore, which is the
			// language's rule rather than a convention -- so `X` and `_Rest` are
			// variables and `foo` is an atom.
			expr(`^[A-Z_][A-Za-z0-9_]*`, groupType, true),
			numbers(),
			symbols(":-", "-->", "?-", "\\+", "@<", "@>", "@=<", "@>=", "=..", "=:=",
				"=\\=", "\\==", "\\=", "==", "=<", ">=", "->", ";", "|",
				"+", "-", "*", "/", "=", "<", ">", "^", "!", "(", ")", "[", "]",
				"{", "}", ",", "."),
		},
	}
}

// backtick is Go's raw-string delimiter, which cannot be written as a raw string
// literal in Go source -- so it gets a function rather than an escape nobody can read.
func backtick() *regexp.Regexp {
	return regexp.MustCompile("^`")
}
