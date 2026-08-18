package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parameter expansion and command substitution, split from expand.go for the
// file-size ceiling. The field-splitting half stays there; this is the half that
// turns one reference into a value.

func (r Runtime) expandParameterPart(ctx context.Context, part wordPart, savedStatus int) []string {
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
		// An array reference produces fields rather than a string, so it is
		// answered here: expandBracedParameter returns one value and cannot say
		// "three words".
		if values, ok := r.expandArrayParameter(ctx, text[2:len(text)-1]); ok {
			return values
		}
		// An operator applied to a list produces fields too, and means something
		// different from the same operator applied to a joined string: `${@:2:2}` is
		// two parameters, not a substring of them. See parameter_list.go.
		if values, ok := r.expandListOperator(ctx, text[2:len(text)-1], savedStatus); ok {
			return values
		}
		expanded, err := r.expandBracedParameter(ctx, text[2:len(text)-1], savedStatus)
		if err != nil {
			r.reportExpansionError(err)
			return []string{""}
		}
		return []string{expanded}
	}
	name := strings.TrimPrefix(text, "$")
	value, set := r.vars[name]
	if !set {
		// The computed names -- $RANDOM, $SECONDS, $PPID -- are not stored, so a
		// bare `$RANDOM` has to ask for them here as well as through the braced
		// path. See special_vars.go.
		if computed, ok := r.dynamicParameter(name); ok {
			return []string{computed}
		}
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
	// `a[0]=value` is an assignment too. applyArrayAssignments takes the ones whose
	// value is a plain literal, before expansion; one whose value comes from an
	// expansion -- `a[0]=$(cmd)` -- reaches here instead, and used to be run as a
	// command called `a[0]=p q`.
	if isArrayElementTarget(name) {
		return true
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
