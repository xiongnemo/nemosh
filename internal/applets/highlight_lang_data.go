package applets

import "regexp"

// JSON, YAML, TOML and Markdown.
//
// The four where the interesting tokens are *structure* rather than keywords, so the
// tables lean on shaped expressions instead of word lists. Markdown is the odd one:
// almost everything in it is line-leading, which makes it the only language here whose
// rules mostly begin with `^\s*`.

func jsonSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "json",
		extensions: []string{".json", ".jsonc", ".jsonl"},
		regions: []highlightRegion{
			quoted(`"`, true),
		},
		patterns: []highlightPattern{
			// JSON proper has no comments; .jsonc does, and colouring one in a plain
			// .json file is more useful than pretending the line is data.
			lineComment("//", groupComment),
			words(groupType, "true", "false", "null"),
			// The exponent and the leading minus are both part of the number in JSON,
			// unlike the shared rule where a minus is an operator.
			expr(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][-+]?[0-9]+)?`, groupNumber, false),
			symbols("{", "}", "[", "]", ",", ":"),
		},
	}
}

func yamlSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "yaml",
		extensions: []string{".yaml", ".yml"},
		regions: []highlightRegion{
			quoted(`"`, true),
			// Single quotes in YAML escape by doubling, not by backslash, so no skip.
			quoted(`'`, false),
		},
		patterns: []highlightPattern{
			lineComment("#", groupComment),
			// A document marker, which is the one thing that must not be read as three
			// operators.
			expr(`^(?:---|\.\.\.)\s*$`, groupKeyword, false),
			// A key, **including its colon**. Go's regexp is RE2 and has no lookahead,
			// so `(?=\s*:)` does not compile -- it panics at MustCompile, which is how
			// this was found. Matching the colon too is the honest way round: it also
			// gets the key colour, which reads fine and is what several editors do.
			//
			// The imperfection this leaves: in `msg: hello: world` the second
			// `hello:` also matches, because a scalar value is not a region and the
			// scan reaches it. Distinguishing them needs a parser rather than a rule.
			expr(`^\s*[A-Za-z_][A-Za-z0-9_.-]*\s*:`, groupKeyword, false),
			// An anchor, an alias, or a tag.
			expr(`^[&*][A-Za-z0-9_-]+`, groupType, false),
			expr(`^!{1,2}[A-Za-z0-9_/:-]*`, groupType, false),
			words(groupType, "true", "false", "null", "yes", "no", "on", "off", "~"),
			numbers(),
			symbols("- ", ": ", ">", "|", "{", "}", "[", "]", ",", ":", "-"),
		},
	}
}

func tomlSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "toml",
		extensions: []string{".toml"},
		regions: []highlightRegion{
			// The triple forms before the single ones, for the reason Python's need it:
			// `"""` begins with `"`, so a shorter rule tried first would take one
			// character and misread the rest.
			{start: regexp.MustCompile(`^"""`), end: regexp.MustCompile(`^"""`), group: groupString},
			{start: regexp.MustCompile(`^'''`), end: regexp.MustCompile(`^'''`), group: groupString},
			quoted(`"`, true),
			// A TOML literal string has no escapes, which is its whole purpose --
			// Windows paths are written in one.
			quoted(`'`, false),
		},
		patterns: []highlightPattern{
			lineComment("#", groupComment),
			// A table header owns its line, brackets included: `[[a.b]]` is one thing.
			expr(`^\s*\[\[?[^\]]*\]\]?`, groupKeyword, false),
			// A key and its equals sign. Written this way for the same reason as
			// YAML's above: RE2 has no lookahead.
			expr(`^\s*[A-Za-z0-9_.-]+\s*=`, groupType, false),
			words(groupType, "true", "false"),
			// A date or a time, which TOML has as first-class values and which the
			// shared number rule would break into pieces.
			expr(`^\d{4}-\d{2}-\d{2}(?:[Tt ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[Zz]|[-+]\d{2}:\d{2})?)?`,
				groupNumber, false),
			numbers(),
			symbols("=", "[", "]", "{", "}", ",", "."),
		},
	}
}

func markdownSyntax() *highlightSyntax {
	return &highlightSyntax{
		name:       "markdown",
		extensions: []string{".md", ".markdown", ".mdown", ".mkd"},
		regions: []highlightRegion{
			// A fenced code block, which is the one construct here that spans lines.
			// The end has to be a fence too, and the language tag after the opening
			// fence is part of the opener.
			{
				start: regexp.MustCompile("^```[A-Za-z0-9_+-]*"),
				end:   regexp.MustCompile("^```"),
				group: groupString,
			},
			{start: regexp.MustCompile("^~~~[A-Za-z0-9_+-]*"), end: regexp.MustCompile("^~~~"), group: groupString},
		},
		patterns: []highlightPattern{
			// A heading owns its line.
			expr(`^\s{0,3}#{1,6}\s.*`, groupKeyword, false),
			// A setext underline, which is a heading written the other way.
			expr(`^(?:=+|-{2,})\s*$`, groupKeyword, false),
			// A block quote marker and a list bullet, both line-leading.
			expr(`^\s*>+`, groupComment, false),
			expr(`^\s*(?:[-*+]|[0-9]+\.)\s`, groupSymbol, false),
			// A thematic break, before the bullet rule could take its first character.
			expr(`^\s*(?:\*\s*){3,}$`, groupSymbol, false),
			expr(`^\s*(?:-\s*){3,}$`, groupSymbol, false),
			// Inline code, then emphasis. Code first, because a backtick span may
			// contain asterisks that are not emphasis.
			expr("^`[^`]*`", groupString, false),
			expr(`^\*\*[^*]+\*\*`, groupType, false),
			expr(`^__[^_]+__`, groupType, false),
			expr(`^\*[^*]+\*`, groupType, false),
			expr(`^_[^_]+_`, groupType, false),
			// A link or an image: the bracketed text and the target.
			expr(`^!?\[[^\]]*\]\([^)]*\)`, groupSymbol, false),
			expr(`^<https?://[^>]+>`, groupSymbol, false),
		},
	}
}
