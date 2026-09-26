package runtime

import "strings"

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
	if quote != quoteUnquoted || separators == "" || r.noFieldSplit {
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

// assignedList joins "$@" or an array's `${a[@]}`, quoted or not, into the one value an
// assignment gives it, since nothing in an assignment is split. They were a word each, so
// `x="$@"` assigned the first parameter and ran the second as a command. busybox-w32 joins
// the parameters with IFS's first character; it has no arrays, and bash joins an array's
// elements with a space whatever IFS is. Unquoted $@ and $* are appendUnquotedParameters'.
func (r Runtime) assignedList(part wordPart, values []string) (string, bool) {
	if !r.noFieldSplit || part.quote == quoteSingle {
		return "", false
	}
	if isArrayAtReference(part.text) {
		return strings.Join(values, " "), true
	}
	if positionalList(part.text) == "$@" && part.quote != quoteUnquoted {
		return strings.Join(values, r.starSeparator()), true
	}
	return "", false
}

// defaultFieldSeparators is IFS as the shell starts with it: space, tab and newline.
const defaultFieldSeparators = " \t\n"

// IFS unset means space, tab, and newline. IFS set to the empty string is a
// different thing: it turns field splitting off.
func (r Runtime) fieldSeparators() string {
	if value, set := r.vars["IFS"]; set {
		return value
	}
	return defaultFieldSeparators
}

// appendUnquotedParameters is unquoted `$@` and `$*`: each parameter split by IFS in turn,
// an empty one vanishing, and a field boundary between parameters even when IFS is empty
// -- then the first piece joins whatever came before in the word and the last whatever
// comes after. It kept each parameter whole and kept the empty ones, so `set -- "a b" ""
// c; for x in $@` looped over `a b`, an empty word and `c`, where both references loop
// over `a`, `b` and `c`. In an assignment, where nothing splits, they are one value joined
// by IFS's first character -- busybox's answer for both; bash joins `$@` with a space
// there, and the two agree while IFS is the default.
func (r Runtime) appendUnquotedParameters(fields, values []string, _ string) ([]string, bool) {
	if r.noFieldSplit {
		fields[len(fields)-1] += strings.Join(values, r.starSeparator())
		return fields, true
	}
	separators := r.fieldSeparators()
	var pieces []string
	for _, value := range values {
		if separators == "" {
			if value != "" {
				pieces = append(pieces, value)
			}
			continue
		}
		pieces = append(pieces, splitOnFieldSeparators(value, separators)...)
	}
	if len(pieces) == 0 {
		return fields, false
	}
	fields[len(fields)-1] += pieces[0]
	return append(fields, pieces[1:]...), true
}

// starSeparator is what `"$*"` and the other `*` forms join with: the first character of
// IFS, a space when IFS is unset, and nothing at all when it is empty (POSIX 2.5.2). They
// all joined with a space, so `IFS=,; echo "$*"` -- the ordinary way to make a comma list
// -- printed blanks where both references print commas.
func (r Runtime) starSeparator() string {
	separators, set := r.vars["IFS"]
	if !set {
		return " "
	}
	for _, first := range separators {
		return string(first)
	}
	return ""
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

// expandHomeTilde turns `~`, `~/path`, and the directory-stack forms into paths.
//
// `~+` is the current directory and `~-` the previous one, which is what makes
// `cp file ~-` mean "back where I just was" without typing it. `~N`, `~+N` and `~-N`
// index the stack `dirs` prints, counting from the current directory and from the far end
// respectively -- the same two directions `pushd +N` and `pushd -N` use, so one mental
// model covers both. Added with the stack, because a stack you cannot name positions in
// is a stack you can only walk.
func (r Runtime) expandHomeTilde(value string) string {
	if replaced, ok := r.expandDirectoryTilde(value); ok {
		return replaced
	}
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
