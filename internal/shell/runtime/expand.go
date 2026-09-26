package runtime

import (
	"context"
	"strconv"
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
			// A pattern matching nothing stays exactly as written, which is POSIX.
			// `shopt -s nullglob` asks for the other answer -- the field disappears --
			// which is what makes `for f in *.none` iterate zero times instead of once
			// over the pattern itself.
			if !r.options.nullGlob {
				expanded = append(expanded, field)
			}
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
//
// The parts are expanded in order into a fieldBuilder, which splits what an unquoted
// expansion produced across the whole word; see field_builder.go. A word that comes to no
// field at all disappears rather than becoming one empty field -- which is what makes
// `set -- $empty` leave no positional parameters.
func (r Runtime) expandWordFields(ctx context.Context, item word, savedStatus int) ([]string, []bool) {
	build := r.newFieldBuilder()
	for _, part := range r.tildeParts(item) {
		switch part.kind {
		case wordPartLiteral:
			build.text(part.text, part.quote == quoteUnquoted)
		case wordPartEscaped:
			// Its backslash was removed by the lexer, so its metacharacter is data no
			// matter where it sits.
			build.text(part.text, false)
		case wordPartParameter:
			r.buildParameter(ctx, build, part, savedStatus)
		case wordPartArithmetic:
			// Expanded before evaluated: the evaluator's lexer has no `$`. See
			// arithmetic_expand.go.
			value, err := r.evaluateArithmetic(r.expandArithmeticText(ctx, part.text, savedStatus))
			if err != nil {
				r.reportExpansionError(err)
				return nil, nil
			}
			build.text(strconv.FormatInt(value, 10), false)
		case wordPartProcessSubstitution, wordPartOutputSubstitution:
			if part.kind == wordPartOutputSubstitution {
				build.text(r.expandOutputSubstitution(ctx, part.script, savedStatus), false)
			} else {
				build.text(r.expandProcessSubstitution(ctx, part.script, savedStatus), false)
			}
		case wordPartCommandSubstitution:
			if part.script != nil {
				build.expansion(r.commandSubstitutionScript(ctx, *part.script, savedStatus), part.quote)
			}
		}
	}
	fields, globbable := build.finish()
	if len(fields) == 0 {
		if !item.quotedEmpty {
			return nil, nil
		}
		fields, globbable = []string{""}, []bool{false}
	}
	return fields, globbable
}

// buildParameter adds a parameter expansion to the word being built.
func (r Runtime) buildParameter(ctx context.Context, build *fieldBuilder, part wordPart, savedStatus int) {
	// A default's word is fields of its own; see parameter_default_fields.go.
	if r.buildDefault(ctx, build, part, savedStatus) {
		return
	}
	values := r.expandParameterPart(ctx, part, savedStatus)
	if joined, isList := r.assignedList(part, values); isList {
		build.text(joined, false)
		return
	}
	list := positionalList(part.text)
	switch {
	// Unquoted, `$@`, `$*` and `${a[@]}` are a field per element, each split in turn, an
	// empty one vanishing; see unquotedList.
	case part.quote == quoteUnquoted && (isArrayAtReference(part.text) || list == "$@" || list == "$*"):
		build.unquotedList(values, r.starSeparator())
	// `"${a[@]}"` is one word per element, exactly as `"$@"` is -- which is the whole
	// reason arrays are worth having, since it is the only form that keeps an element
	// containing a blank intact.
	case part.quote != quoteSingle && (isArrayAtReference(part.text) || list == "$@"):
		build.quotedList(values)
	default:
		build.expansion(values[0], part.quote)
	}
}
