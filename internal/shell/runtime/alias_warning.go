package runtime

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// As nemosh and sh an alias is substituted as its command runs (alias_expand.go), so one that is
// part of a command's syntax -- `alias LEFT='{'`, a reserved word, half a loop -- is a syntax error
// where it is used. That is on purpose, as Oils has it: POSIX allows reading an alias so, bash's
// manual and ShellCheck's SC2262 counsel against relying on it, and a script that does is a
// script for bash, which bash mode reads as bash does (alias_read.go). What nemosh does when it
// meets one is say so on a line after the syntax error, so it is not left unexplained.

// sayAliasSyntax follows a syntax error in text with a hint when an alias is why: when the aliases
// defined, with those text defines itself with `alias NAME=VALUE`, substituted as bash reads them,
// make text parse. Nothing in bash mode, which reads them so.
func (r Runtime) sayAliasSyntax(text string) {
	if r.BashMode() || !strings.Contains(text, "alias") && len(r.aliases) == 0 {
		return
	}
	aliases := map[string]string{}
	if r.options != nil && r.options.expandAliases {
		maps.Copy(aliases, r.aliases)
	}
	maps.Copy(aliases, definedAliases(text))
	if len(aliases) == 0 {
		return
	}
	if _, err := parseReading(text, 1, false, aliases, false); err != nil {
		return
	}
	if name, ok := syntaxAlias(text, aliases); ok {
		r.hintAliasSyntax(name, aliases[name])
	}
}

// hintAliasSyntax is the hint itself, for the alias name whose text is value.
func (r Runtime) hintAliasSyntax(name, value string) {
	fmt.Fprintf(r.streams.Stderr, "hint: alias %s=%s is part of a command's syntax, which nemosh does not read on purpose; bash mode does: nemosh bash, or NEMOSH_COMPAT=bash\n",
		name, shellquote.SingleQuoted(value))
}

// syntaxAlias is the alias to name: of those text writes where a command's name could be, the
// first whose text is no whole command on its own, or failing one, the first.
func syntaxAlias(text string, aliases map[string]string) (string, bool) {
	lines := strings.Split(text, "\n")
	first := ""
	for _, name := range slices.Sorted(maps.Keys(aliases)) {
		if !slices.ContainsFunc(lines, func(line string) bool { return containsCommandWord(line, name) }) {
			continue
		}
		if _, err := parseScriptAt(aliases[name], 1); err != nil {
			return name, true
		}
		if first == "" {
			first = name
		}
	}
	return first, first != ""
}

// definedAliases is what the alias commands text holds would define once they ran, each line
// that holds one parsed on its own: found without running any of it.
func definedAliases(text string) map[string]string {
	defined := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "alias") {
			continue
		}
		script, err := parseScriptAt(line, 1)
		if err != nil {
			continue
		}
		for _, node := range script.program {
			if list, ok := node.(listNode); ok {
				aliasDefinitions(list.value, defined)
			}
		}
	}
	return defined
}

// aliasDefinitions adds the definitions of the alias commands in items to defined.
func aliasDefinitions(items list, defined map[string]string) {
	for _, item := range items.items {
		for _, pipeline := range item.value.pipelines {
			for _, command := range pipeline.commands {
				simple, ok := command.(simpleCommand)
				if !ok || len(simple.words) == 0 {
					continue
				}
				if name, _ := plainWord(simple.words[0]); name != "alias" {
					continue
				}
				for _, operand := range simple.words[1:] {
					text, ok := literalText(operand)
					if name, value, found := strings.Cut(text, "="); ok && found && name != "" {
						defined[name] = value
					}
				}
			}
		}
	}
}

// literalText is a word's text with its quotes taken off, when it expands nothing.
func literalText(item word) (string, bool) {
	var text strings.Builder
	for _, part := range item.parts {
		if part.kind != wordPartLiteral && part.kind != wordPartEscaped {
			return "", false
		}
		text.WriteString(part.text)
	}
	return text.String(), true
}
