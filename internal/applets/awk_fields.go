package applets

import (
	"strconv"
	"strings"
)

// `$0`, the fields, and NF.
//
// The three move together and each can be assigned, so the rule is that **whichever was
// written last is the truth** and the other is rebuilt when someone asks. Doing it eagerly
// would re-split the record every time a program reads `$1`.
//
// Field splitting follows POSIX exactly, because the special cases are what scripts rely
// on and all four were measured:
//
//	FS = " "    the default: split on runs of blanks, and strip leading and trailing ones
//	FS = ","    a single character: literal, and empty fields are kept -- `a,,b` is three
//	FS = "\t"   a single tab, same rule
//	FS = "[,;]" more than one character: an extended regular expression
//
// The first is not the same as `FS = "[ ]"`, and that is the point: with the default,
// `"  a  b  "` is two fields, while a single-space FS would make it six.

// setRecord replaces `$0` and marks the fields for re-splitting.
func (in *awkInterp) setRecord(text string) {
	in.record = text
	in.fieldsStale = true
	in.recordStale = false
}

// getRecord answers `$0`, rebuilding it from the fields if one was assigned.
func (in *awkInterp) getRecord() string {
	in.ensureRecord()
	return in.record
}

// ensureFields splits `$0` if it has moved since the fields were last built.
func (in *awkInterp) ensureFields() {
	if !in.fieldsStale {
		return
	}
	in.fieldsStale = false
	in.fields = in.splitRecord(in.record)
}

// ensureRecord rebuilds `$0` from the fields if one was assigned.
func (in *awkInterp) ensureRecord() {
	if !in.recordStale {
		return
	}
	in.recordStale = false
	in.record = strings.Join(in.fields, in.vars["OFS"].str(in.convfmt()))
}

// splitRecord applies the FS rules.
func (in *awkInterp) splitRecord(text string) []string {
	separator := in.vars["FS"].str(in.convfmt())
	return awkSplitFields(text, separator)
}

// awkSplitFields is the splitting rule on its own, so that `split()` can share it.
func awkSplitFields(text, separator string) []string {
	switch {
	case text == "":
		// An empty record has no fields at all, whatever FS says, so NF is 0 rather
		// than 1. Both references agree.
		return nil
	case separator == " ":
		// The default: runs of blanks, with leading and trailing ones discarded.
		return strings.FieldsFunc(text, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n'
		})
	case len(separator) == 1 && separator != "\\":
		// A single character is a literal, and empty fields survive: `a,,b` is three.
		return strings.Split(text, separator)
	}
	compiled, err := compileAwkRegex(separator)
	if err != nil {
		// A bad FS falls back to treating it literally rather than failing the record,
		// which is what busybox does and is the less destructive answer: a program with
		// a malformed FS still sees its input.
		return strings.Split(text, separator)
	}
	return compiled.Split(text, -1)
}

// getField answers `$n`.
func (in *awkInterp) getField(index int) awkValue {
	if index == 0 {
		// `$0` is a strnum like any other field, so a record of `10` compares
		// numerically.
		return awkStrnumOf(in.getRecord())
	}
	in.ensureFields()
	if index < 1 || index > len(in.fields) {
		// Past the end is the empty string and *uninitialised*, not an empty strnum:
		// `$99 == 0` is true on a short record, which is the uninitialised rule.
		return awkValue{}
	}
	return awkStrnumOf(in.fields[index-1])
}

// setField assigns `$n`, extending the record if n is past NF.
func (in *awkInterp) setField(index int, text string) {
	if index == 0 {
		in.setRecord(text)
		return
	}
	in.ensureFields()
	for len(in.fields) < index {
		// Assigning past NF fills the gap with empty fields, so `$5="E"` on a
		// three-field record gives `a b c  E` -- two spaces, because `$4` is empty.
		in.fields = append(in.fields, "")
	}
	in.fields[index-1] = text
	in.recordStale = true
}

// setFieldCount is `NF = n`, which truncates or extends.
func (in *awkInterp) setFieldCount(count int) {
	in.ensureFields()
	if count < 0 {
		count = 0
	}
	for len(in.fields) < count {
		in.fields = append(in.fields, "")
	}
	in.fields = in.fields[:count]
	in.recordStale = true
}

// fieldIndex turns an evaluated subscript into a field number, refusing a negative one.
//
// Truncated rather than rounded, which is what awk does with every number used as an
// index: `$1.7` is `$1`.
func (in *awkInterp) fieldIndex(value awkValue) (int, error) {
	number := value.num()
	index := int(number)
	if index < 0 {
		return 0, in.errorf("attempt to access field %s", strconv.Itoa(index))
	}
	return index, nil
}
