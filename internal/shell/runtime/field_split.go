package runtime

import "strings"

// assignedList joins "$@" or an array's `${a[@]}`, quoted or not, into the one value an
// assignment gives it, since nothing in an assignment is split. They were a word each, so
// `x="$@"` assigned the first parameter and ran the second as a command. busybox-w32 joins
// the parameters with IFS's first character; it has no arrays, and bash joins an array's
// elements with a space whatever IFS is. Unquoted $@ and $* are fieldBuilder.unquotedList's.
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
