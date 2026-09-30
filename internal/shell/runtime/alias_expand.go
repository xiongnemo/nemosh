package runtime

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// An alias is read as shell text, as busybox and bash read it: its value where the command
// name was, and the rest of the command after it as written, parsed again and run. So a value
// may hold whatever a command line may -- `;`, `&&`, a pipe, a redirection, a newline -- its
// `$HOME` is expanded where it is used, and the rest of the command joins its last command:
// with `alias e_='for i in 1 2 3; do echo $i;'`, `e_ done` is a loop.
//
// It was words, put in the command name's place once the command had been expanded, and a
// value that was not a list of words was refused when it was defined: `$HOME` stayed as
// written, and `alias x='echo one; echo two'` could not be had at all.
//
// The references substitute as they read a line; this substitutes as the command runs, since
// a script is parsed in full before any of it runs, from the aliases its line was read with
// (alias_view.go). A value holding an operator keeps to its own command's place, so in `x | wc
// -l` all of x's commands are piped where only its last is in theirs, and a redirection written
// before the name is read after the rest, since the parse keeps no order between the two.
//
// A name is not substituted again inside its own value, nor inside what that value runs, which
// is what lets `alias ls='ls -F'` mean what it says and stops `alias a=b b=a`.

// runAlias runs a simple command whose name is an alias as the text it stands for, and answers
// false, having done nothing, for any other command. A value that does not parse is a syntax
// error, as it is in both references: the end of a script, and of the line at a prompt.
func (r Runtime) runAlias(ctx context.Context, command simpleCommand, savedStatus int) (lineResult, bool) {
	text, names, ok := r.aliasText(command)
	if !ok {
		return lineResult{}, false
	}
	r.enterLine(command.line)
	script, err := r.parseHere(text)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return lineResult{status: 2, control: flowAbort}, true
	}
	for _, name := range names {
		if !slices.Contains(r.aliasChain, name) {
			r.aliasChain = append(slices.Clip(r.aliasChain), name)
		}
	}
	status, control := r.executeProgram(ctx, script.program, savedStatus)
	return lineResult{status: status, control: control}, true
}

// isAliasCommand reports whether a simple command's name is an alias that runAlias would run.
func (r Runtime) isAliasCommand(command simpleCommand) bool {
	_, _, ok := r.aliasAt(command.words, commandNameIndex(command.words))
	return ok
}

// aliasText is the text an aliased command stands for, and the aliases it took; see runAlias.
// What was written in front of the name, assignments and redirections, stays in front of the
// value: `> log x` sends x's first command to log, and `x > log` its last.
func (r Runtime) aliasText(command simpleCommand) (string, []string, bool) {
	words := command.words
	index := commandNameIndex(words)
	value, name, ok := r.aliasAt(words, index)
	if !ok {
		return "", nil, false
	}
	var leading, trailing []redirectOperation
	for _, operation := range command.redirects {
		if operation.words <= index {
			leading = append(leading, operation)
		} else {
			trailing = append(trailing, operation)
		}
	}
	var printer scriptPrinter
	var text strings.Builder
	text.WriteString(printer.command(simpleCommand{words: words[:index], redirects: leading}) + " " + value)
	names := []string{name}
	// A value that ends in a blank makes the next word an alias's too, which is what
	// `alias sudo='sudo '` is for, and one that ends where a command begins makes it a command
	// name. Either way it is looked up with this value behind it, as the references look it up
	// once the value is used: with `alias x='echo a;'`, `x x` is x twice, where an x inside x's
	// own value is only a command name.
	index++
	for opens := opensNextWord(value); opens; index++ {
		next, nextName, found := r.aliasAt(words, index)
		if !found {
			break
		}
		read, taken, after := r.rereadAlias(next, []string{nextName})
		text.WriteString(" " + read)
		names = append(append(names, nextName), taken...)
		opens = after
	}
	rest := printer.command(simpleCommand{words: words[index:], redirects: trailing})
	printer.line(text.String() + " " + rest)
	return printer.out.String(), names, true
}

// aliasAt is the value of the alias the index'th word names, if it is an unquoted word naming
// one that this command is not already inside.
func (r Runtime) aliasAt(words []word, index int) (string, string, bool) {
	aliases := r.aliasesInForce()
	if !r.options.expandAliases || len(aliases) == 0 || index >= len(words) || !isUnquotedLiteralWord(words[index]) {
		return "", "", false
	}
	name := words[index].parts[0].text
	for _, part := range words[index].parts[1:] {
		name += part.text
	}
	value, defined := aliases[name]
	if !defined || slices.Contains(r.aliasChain, name) || len(r.aliasChain) >= maxAliasSubstitutions {
		return "", "", false
	}
	return value, name, true
}

// commandNameIndex is where a simple command's name is: past the assignments in front of it.
func commandNameIndex(words []word) int {
	index := 0
	for index < len(words) && isAssignmentWord(words[index]) {
		index++
	}
	return index
}

// opensNextWord reports whether the word after an alias's value is read as an alias too: after
// a trailing blank, and after a value that leaves a command to begin -- one that is empty, or
// ends in an operator or a newline that is not escaped.
func opensNextWord(value string) bool {
	if value == "" || endsWithBlank(value) {
		return true
	}
	escaped := len(value) > 1 && value[len(value)-2] == '\\'
	return strings.IndexByte(";&|(\n", value[len(value)-1]) >= 0 && !escaped
}

// rereadAlias is the value of an alias that follows a value ending in a blank, read as the
// references read it: its first word is looked up again, and so is the word after any alias in
// it that ends in a blank, while each names an alias not already being read. With `alias
// foo='echo ' bar=baz baz=quux`, `foo bar` is echo quux; the value went in as it was, since only
// a command's name is looked up once the text is parsed again, and it was echo baz. It answers
// the text, the aliases it took, and whether the word after the value is looked up too.
func (r Runtime) rereadAlias(value string, inUse []string) (string, []string, bool) {
	var out strings.Builder
	var taken []string
	opens, rest := opensNextWord(value), value
	for check := true; check; {
		start, end, plain := plainLeadingWord(rest)
		name := rest[start:end]
		next, defined := r.aliasesInForce()[name]
		if !plain || !defined || slices.Contains(inUse, name) || slices.Contains(r.aliasChain, name) ||
			len(inUse)+len(r.aliasChain) >= maxAliasSubstitutions {
			break
		}
		read, more, after := r.rereadAlias(next, append(slices.Clip(inUse), name))
		out.WriteString(rest[:start] + read)
		taken = append(append(taken, name), more...)
		rest, check = rest[end:], after
		if rest == "" {
			opens = after
		}
	}
	out.WriteString(rest)
	return out.String(), taken, opens
}

// plainLeadingWord is where the first word of text begins and ends, past its blanks, and
// whether it is one an alias can name: no quote, backslash or expansion in it. A quoted word
// ends the looking up, as it does in busybox's: with `alias x='"y" w'`, `echo x` after a value
// ending in a blank is y w.
func plainLeadingWord(text string) (int, int, bool) {
	start := len(text) - len(strings.TrimLeft(text, " \t"))
	end := strings.IndexAny(text[start:], " \t\n;&|()<>")
	if end < 0 {
		end = len(text) - start
	}
	word := text[start : start+end]
	return start, start + end, word != "" && !strings.ContainsAny(word, "'\"\\$`")
}
