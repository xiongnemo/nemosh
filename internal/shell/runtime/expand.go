package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// expandCommandWord is expandWord followed by the pathname expansion of POSIX
// 2.6.6. It is separate because that step applies to a command's words, its
// redirect operands, and a for-loop's word list, but never to a case pattern --
// there the pattern is the whole point, and globbing `*)` against the
// filesystem would take the default arm away.
func (r Runtime) expandCommandWord(ctx context.Context, item word, savedStatus int) []string {
	// Brace expansion first, and it is the only expansion that turns one word
	// into several *before* anything is looked up. See brace.go: with `x=1`,
	// `echo {$x,2}` prints `1 2`, so the split has to happen while the parameter
	// is still unexpanded.
	var expanded []string
	for _, braced := range expandBraceWord(item) {
		expanded = append(expanded, r.expandOneCommandWord(ctx, braced, savedStatus)...)
	}
	return expanded
}

func (r Runtime) expandOneCommandWord(ctx context.Context, item word, savedStatus int) []string {
	fields, globbable := r.expandWordFields(ctx, item, savedStatus)
	var expanded []string
	for index, field := range fields {
		if !globbable[index] {
			expanded = append(expanded, field)
			continue
		}
		matches := r.expandPathnames(field)
		if len(matches) == 0 {
			// A pattern matching nothing stays exactly as written.
			expanded = append(expanded, field)
			continue
		}
		expanded = append(expanded, matches...)
	}
	return expanded
}

func (r Runtime) expandWord(ctx context.Context, item word, savedStatus int) []string {
	fields, _ := r.expandWordFields(ctx, item, savedStatus)
	return fields
}

// expandWordFields returns the fields and, for each of them, whether an
// unquoted part contributed a pathname metacharacter to it. Quoting is what
// decides: `echo "*"` prints a star and `echo *` lists the directory, and the
// only thing that tells them apart is where the star came from.
func (r Runtime) expandWordFields(ctx context.Context, item word, savedStatus int) ([]string, []bool) {
	fields := []string{""}
	globbable := []bool{false}
	// mark records that an unquoted contribution carrying a metacharacter
	// landed on every field from `from` onwards -- a split expansion can add
	// several at once.
	mark := func(text string, quote quoteContext, from int) {
		if quote != quoteUnquoted || !containsGlobMeta(text) {
			return
		}
		for len(globbable) < len(fields) {
			globbable = append(globbable, false)
		}
		for index := from; index < len(globbable); index++ {
			globbable[index] = true
		}
	}
	// contributed tracks whether anything at all put a field on the word. An
	// unquoted expansion that splits to nothing puts nothing, and a word made
	// only of those disappears rather than becoming one empty field -- which is
	// what makes `set -- $empty` leave no positional parameters.
	contributed := false
	for _, part := range item.parts {
		start := len(fields) - 1
		switch part.kind {
		case wordPartLiteral, wordPartEscaped:
			fields[len(fields)-1] += part.text
			contributed = true
			// An escaped part had its backslash removed by the lexer, so its
			// metacharacter is data no matter where it sits.
			if part.kind == wordPartLiteral {
				mark(part.text, part.quote, start)
			}
		case wordPartParameter:
			values := r.expandParameterPart(part, savedStatus)
			if part.text == "$@" && part.quote != quoteSingle {
				if len(values) == 0 {
					if len(item.parts) == 1 {
						return nil, nil
					}
					continue
				}
				fields[len(fields)-1] += values[0]
				fields = append(fields, values[1:]...)
				contributed = true
				mark(strings.Join(values, ""), part.quote, start)
				continue
			}
			var produced bool
			fields, produced = r.appendExpansion(fields, values[0], part.quote)
			contributed = contributed || produced
			mark(values[0], part.quote, start)
		case wordPartArithmetic:
			value, err := r.evaluateArithmetic(part.text)
			if err != nil {
				r.reportExpansionError(err)
				return nil, nil
			}
			fields[len(fields)-1] += strconv.FormatInt(value, 10)
			contributed = true
		case wordPartCommandSubstitution:
			if part.script != nil {
				output := r.commandSubstitutionScript(ctx, *part.script, savedStatus)
				var produced bool
				fields, produced = r.appendExpansion(fields, output, part.quote)
				contributed = contributed || produced
				mark(output, part.quote, start)
			}
		}
	}
	globbable = append(globbable, make([]bool, len(fields)-len(globbable))...)
	if len(item.parts) == 0 && !item.quotedEmpty {
		return nil, nil
	}
	if !contributed && !item.quotedEmpty {
		return nil, nil
	}
	if item.expandTilde && len(fields) > 0 {
		fields[0] = r.expandHomeTilde(fields[0])
	}
	return fields, globbable
}

// appendExpansion adds what an expansion produced to the word being built. An
// unquoted result is split into fields on IFS (POSIX 2.6.5) and a quoted one is
// appended whole. Nothing split before, so `set -- $(echo a b)` left one
// positional parameter holding both words, and `for f in $list` looped once
// over the whole list.
// The second result reports whether a field was contributed: a quoted
// expansion always contributes one even when it is empty, and an unquoted one
// contributes nothing when it splits to nothing.
func (r Runtime) appendExpansion(fields []string, value string, quote quoteContext) ([]string, bool) {
	separators := r.fieldSeparators()
	if quote != quoteUnquoted || separators == "" {
		fields[len(fields)-1] += value
		return fields, true
	}
	pieces := splitOnFieldSeparators(value, separators)
	if len(pieces) == 0 {
		return fields, false
	}
	fields[len(fields)-1] += pieces[0]
	return append(fields, pieces[1:]...), true
}

// IFS unset means space, tab, and newline. IFS set to the empty string is a
// different thing: it turns field splitting off.
func (r Runtime) fieldSeparators() string {
	if value, set := r.vars["IFS"]; set {
		return value
	}
	return " \t\n"
}

// A run of IFS whitespace is one delimiter and a leading or trailing run makes
// no empty field, while a non-whitespace separator delimits one field each --
// which is why `IFS=:` over `a::b` gives three fields and the middle one is
// empty.
func splitOnFieldSeparators(value, separators string) []string {
	var fields []string
	var current strings.Builder
	started := false
	for index := 0; index < len(value); index++ {
		char := value[index]
		if strings.IndexByte(separators, char) < 0 {
			current.WriteByte(char)
			started = true
			continue
		}
		if char == ' ' || char == '\t' || char == '\n' {
			if started {
				fields = append(fields, current.String())
				current.Reset()
				started = false
			}
			continue
		}
		fields = append(fields, current.String())
		current.Reset()
		started = false
	}
	if started {
		fields = append(fields, current.String())
	}
	return fields
}

func (r Runtime) expandHomeTilde(value string) string {
	if value != "~" && !strings.HasPrefix(value, "~/") {
		return value
	}
	home := r.vars["HOME"]
	if home == "" {
		home = r.vars["USERPROFILE"]
	}
	if home == "" {
		return value
	}
	if value == "~" {
		return home
	}
	return strings.TrimRight(home, `/\`) + "/" + strings.TrimPrefix(value, "~/")
}

func (r Runtime) expandParameterPart(part wordPart, savedStatus int) []string {
	text := part.text
	switch text {
	case "$0":
		return []string{r.params.name}
	case "$?":
		return []string{strconv.Itoa(savedStatus)}
	case "$#":
		return []string{strconv.Itoa(len(r.params.values))}
	case "$@":
		return append([]string(nil), r.params.values...)
	case "$*":
		return []string{strings.Join(r.params.values, " ")}
	case "$-":
		return []string{r.options.letters()}
	}
	if len(text) == 2 && '1' <= text[1] && text[1] <= '9' {
		index := int(text[1] - '1')
		if index < len(r.params.values) {
			return []string{r.params.values[index]}
		}
		r.reportUnsetParameter(text[1:])
		return []string{""}
	}
	if strings.HasPrefix(text, "${") && strings.HasSuffix(text, "}") {
		expanded, err := r.expandBracedParameter(text[2:len(text)-1], savedStatus)
		if err != nil {
			r.reportExpansionError(err)
			return []string{""}
		}
		return []string{expanded}
	}
	name := strings.TrimPrefix(text, "$")
	value, set := r.vars[name]
	if !set {
		r.reportUnsetParameter(name)
	}
	return []string{value}
}

func (r Runtime) commandSubstitutionScript(ctx context.Context, script Script, savedStatus int) string {
	var stdout bytes.Buffer
	child, err := r.snapshot(ctx)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return ""
	}
	table := child.fds
	if err := table.bindBorrowedWriter(1, &stdout); err != nil {
		child.jobScope.cancelAndDrain()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, table.closeAll()))
		return ""
	}
	child = child.withFDTable(table)
	child.traps = map[trapName]string{}
	child.executeTypedScriptFrom(ctx, script, savedStatus)
	child.jobScope.cancelAndDrain()
	if err := table.closeAll(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return ""
	}
	return strings.TrimRight(stdout.String(), "\n")
}

func isAssignment(arg string) bool {
	name, _, ok := strings.Cut(arg, "=")
	if !ok || name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isNameByte(name[i]) {
			return false
		}
	}
	return true
}

func isNameByte(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9') || b == '_'
}
