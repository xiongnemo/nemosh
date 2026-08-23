package applets

// Go, C and C++.
//
// Grouped because they share the shape -- `//` and `/* */`, double-quoted strings with
// backslash escapes, a keyword list and a type list -- and differ only in the lists.
// Keeping them together is what makes the differences visible: C++ is C's list plus
// twenty words, and Go's `rune` and `chan` are the two nobody else has.

func goSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "go",
		extensions: []string{".go"},
		regions: []highlightRegion{
			blockComment("/*", "*/", false),
			quoted(`"`, true),
			// A raw string, where a backslash is a backslash and only a second
			// backtick ends it. No skip, deliberately: `\` inside one is literal, so
			// treating it as an escape would let the string swallow the closer.
			{start: backtick(), end: backtick(), group: groupString},
		},
		patterns: []highlightPattern{
			lineComment("//", groupComment),
			// A rune literal, as a bounded pattern rather than a region: a single
			// quote also appears in prose inside comments, and a region would run
			// away. One character or one escape, then the closer.
			expr(`^'(?:\\.|[^'\\])'`, groupString, false),
			words(groupKeyword,
				"break", "case", "chan", "const", "continue", "default", "defer", "else",
				"fallthrough", "for", "func", "go", "goto", "if", "import", "interface",
				"map", "package", "range", "return", "select", "struct", "switch", "type", "var"),
			words(groupType,
				"bool", "byte", "complex64", "complex128", "error", "float32", "float64",
				"int", "int8", "int16", "int32", "int64", "rune", "string",
				"uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "any",
				"true", "false", "nil", "iota", "make", "new", "len", "cap", "append",
				"copy", "delete", "panic", "recover", "print", "println", "close", "min", "max"),
			numbers(),
			symbols(":=", "...", "<-", "&&", "||", "==", "!=", "<=", ">=", "++", "--",
				"+", "-", "*", "/", "%", "=", "<", ">", "!", "&", "|", "^", "(", ")",
				"{", "}", "[", "]", ",", ";", ":", "."),
		},
	}
}

func cSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "c",
		extensions: []string{".c", ".h"},
		regions:    cFamilyRegions(),
		patterns: append(cFamilyLeadingPatterns(),
			words(groupKeyword, cKeywords()...),
			words(groupType, cTypes()...),
			numbers(),
			cFamilySymbols()),
	}
}

func cppSyntax() *highlightSyntax {
	return &highlightSyntax{
		name: "c++",
		// `.h` belongs to C above, which wins because the two lists are searched in
		// order and a header is more often C. `.hpp` and `.hh` are unambiguous.
		extensions: []string{".cc", ".cpp", ".cxx", ".hpp", ".hh", ".c++"},
		regions:    cFamilyRegions(),
		patterns: append(cFamilyLeadingPatterns(),
			words(groupKeyword, append(cKeywords(),
				"class", "namespace", "template", "typename", "public", "private", "protected",
				"virtual", "override", "final", "explicit", "friend", "operator", "this",
				"new", "delete", "try", "catch", "throw", "using", "constexpr", "consteval",
				"decltype", "noexcept", "nullptr", "concept", "requires", "co_await",
				"co_return", "co_yield", "static_assert", "dynamic_cast", "static_cast",
				"const_cast", "reinterpret_cast", "mutable", "export", "import", "module")...),
			words(groupType, append(cTypes(),
				"bool", "true", "false", "wchar_t", "char8_t", "char16_t", "char32_t",
				"string", "vector", "map", "set", "pair", "optional", "variant",
				"unique_ptr", "shared_ptr", "weak_ptr", "size_t", "ptrdiff_t")...),
			numbers(),
			cFamilySymbols()),
	}
}

// cFamilyRegions are the two multi-line constructs C and C++ share.
func cFamilyRegions() []highlightRegion {
	return []highlightRegion{
		blockComment("/*", "*/", false),
		quoted(`"`, true),
	}
}

// cFamilyLeadingPatterns are the rules that must be tried before the keyword lists.
//
// The preprocessor line has to come first: `#include <stdio.h>` would otherwise have
// its `<stdio.h>` read as two operators and a keyword-shaped word, and the `#` itself
// matches nothing at all.
func cFamilyLeadingPatterns() []highlightPattern {
	return []highlightPattern{
		lineComment("//", groupComment),
		// A preprocessor directive owns its line, which is the honest simplification:
		// a full C preprocessor is not a highlighting problem.
		expr(`^\s*#\s*[a-z_]+.*`, groupKeyword, false),
		// A character literal, bounded rather than a region, for the same reason Go's
		// rune literal is: an apostrophe in a comment must not open a string.
		expr(`^'(?:\\.|[^'\\])'`, groupString, false),
	}
}

func cFamilySymbols() highlightPattern {
	return symbols("->", "<<=", ">>=", "<<", ">>", "&&", "||", "==", "!=", "<=", ">=",
		"++", "--", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "::",
		"+", "-", "*", "/", "%", "=", "<", ">", "!", "&", "|", "^", "~", "?",
		"(", ")", "{", "}", "[", "]", ",", ";", ":", ".")
}

func cKeywords() []string {
	return []string{
		"auto", "break", "case", "const", "continue", "default", "do", "else", "enum",
		"extern", "for", "goto", "if", "inline", "register", "restrict", "return",
		"sizeof", "static", "struct", "switch", "typedef", "union", "volatile", "while",
		"_Alignas", "_Alignof", "_Atomic", "_Generic", "_Noreturn", "_Static_assert",
		"_Thread_local", "alignas", "alignof", "thread_local",
	}
}

func cTypes() []string {
	return []string{
		"char", "double", "float", "int", "long", "short", "signed", "unsigned", "void",
		"_Bool", "_Complex", "_Imaginary", "int8_t", "int16_t", "int32_t", "int64_t",
		"uint8_t", "uint16_t", "uint32_t", "uint64_t", "NULL",
	}
}
