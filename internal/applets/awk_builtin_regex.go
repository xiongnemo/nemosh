package applets

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// The built-ins that take a regular expression: `split`, `sub`, `gsub` and `match`.
//
// Three rules, each measured against both references:
//
//   - **A regex literal is always a regex; a string may not be.** `split(s, a, ".")` uses a
//     literal dot -- the single-character FS rule -- while `split(s, a, /./)` splits on
//     every character. For `a.b.c` the two answer 3 and 6, so the parser's distinction
//     between a regex node and a string has to survive as far as here.
//   - **An empty match abutting the previous one is skipped.** `gsub(/a*/, "-", "aaa")`
//     replaces once, not twice: gawk answers 1 and busybox 2, and gawk is right. Go's
//     `FindAllStringIndex` already carries exactly that rule, and using it also keeps `^`
//     and `$` anchored to the whole string -- rescanning a re-sliced remainder would let
//     `^` match in the middle of the text, which is the bug this shape avoids rather than
//     has to remember.
//   - **In the replacement, `&` is the matched text and `\&` a literal one**, and `\\` is a
//     single backslash. That is POSIX, and busybox; gawk alone turns a `\\` that no `&`
//     follows into two backslashes.

func (in *awkInterp) builtinSplit(node awkBuiltinExpr) (awkValue, error) {
	text, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	name, ok := node.args[1].(awkVarExpr)
	if !ok {
		return awkValue{}, in.errorf("split needs an array name as its second argument")
	}
	parts, err := in.splitFor(text, node)
	if err != nil {
		return awkValue{}, err
	}
	// The array is emptied first: `split` replaces its target rather than adding to it.
	// In place, so that splitting into a parameter refills the caller's array.
	array := in.getArray(name.name)
	array.clear()
	for index, part := range parts {
		// The pieces are **strnums**, like fields, so `split("1 2", a); a[1] == 1` holds.
		array.set(strconv.Itoa(index+1), awkStrnumOf(part))
	}
	return awkNum(float64(len(parts))), nil
}

// splitFor answers the pieces, choosing between the FS rules and a plain regular expression.
func (in *awkInterp) splitFor(text string, node awkBuiltinExpr) ([]string, error) {
	if len(node.args) < 3 {
		// No separator means FS, which is what makes `split($0, a)` mirror the fields.
		return in.splitRecord(text), nil
	}
	if regex, ok := node.args[2].(awkRegexExpr); ok {
		compiled, err := compileAwkRegex(regex.pattern)
		if err != nil {
			return nil, err
		}
		if text == "" {
			return nil, nil
		}
		return compiled.Split(text, -1), nil
	}
	separator, err := in.argText(node, 2)
	if err != nil {
		return nil, err
	}
	return awkSplitFields(text, separator), nil
}

// builtinSub is `sub` and `gsub`, which differ only in how many matches they take.
//
// The target defaults to `$0` and must be assignable, so `gsub(/x/, "y", $2)` rewrites the
// field and rebuilds the record. It is written back **only when something matched**: an
// untouched field keeps being a strnum, where assigning its own text back would quietly
// make it a string and change how it compares.
func (in *awkInterp) builtinSub(node awkBuiltinExpr, global bool) (awkValue, error) {
	pattern, err := in.patternOf(node.args[0])
	if err != nil {
		return awkValue{}, err
	}
	compiled, err := compileAwkRegex(pattern)
	if err != nil {
		return awkValue{}, err
	}
	replacement, err := in.argText(node, 1)
	if err != nil {
		return awkValue{}, err
	}
	target := awkExpr(awkFieldExpr{index: awkNumberExpr{value: 0}})
	if len(node.args) > 2 {
		target = node.args[2]
	}
	current, err := in.loadLvalue(target)
	if err != nil {
		return awkValue{}, err
	}
	updated, count := awkSubstitute(compiled, current.str(in.convfmt()), replacement, global)
	if count > 0 {
		if err := in.storeLvalue(target, awkStr(updated)); err != nil {
			return awkValue{}, err
		}
	}
	return awkNum(float64(count)), nil
}

// awkSubstitute rewrites every match, or just the first, and reports how many there were.
func awkSubstitute(compiled *regexp.Regexp, text, replacement string, global bool) (string, int) {
	var spans [][]int
	if global {
		spans = compiled.FindAllStringIndex(text, -1)
	} else if span := compiled.FindStringIndex(text); span != nil {
		spans = [][]int{span}
	}
	if len(spans) == 0 {
		return text, 0
	}
	var out strings.Builder
	last := 0
	for _, span := range spans {
		out.WriteString(text[last:span[0]])
		out.WriteString(awkExpandReplacement(replacement, text[span[0]:span[1]]))
		last = span[1]
	}
	out.WriteString(text[last:])
	return out.String(), len(spans)
}

// awkExpandReplacement puts the matched text where the replacement asks for it.
//
// The escapes were already decoded when the string was lexed, so what arrives here is the
// characters themselves: a source `"[\\&]"` is the three-character replacement `[\&]`, and
// this is where that becomes a literal `&`.
func awkExpandReplacement(replacement, matched string) string {
	var out strings.Builder
	for index := 0; index < len(replacement); index++ {
		character := replacement[index]
		if character == '&' {
			out.WriteString(matched)
			continue
		}
		if character != '\\' || index+1 >= len(replacement) {
			// A trailing backslash stands for itself, which both references agree on.
			out.WriteByte(character)
			continue
		}
		switch replacement[index+1] {
		case '&':
			out.WriteByte('&')
			index++
		case '\\':
			out.WriteByte('\\')
			index++
		default:
			out.WriteByte('\\')
		}
	}
	return out.String()
}

// builtinMatch finds a pattern and records where it was.
//
// RSTART and RLENGTH are the answer as much as the return value is, and **RLENGTH is -1**
// rather than 0 when nothing matched -- that is what a program tests. Both are in runes,
// so they line up with what `substr` would then take.
func (in *awkInterp) builtinMatch(node awkBuiltinExpr) (awkValue, error) {
	text, err := in.argText(node, 0)
	if err != nil {
		return awkValue{}, err
	}
	pattern, err := in.patternOf(node.args[1])
	if err != nil {
		return awkValue{}, err
	}
	compiled, err := compileAwkRegex(pattern)
	if err != nil {
		return awkValue{}, err
	}
	span := compiled.FindStringIndex(text)
	if span == nil {
		in.vars["RSTART"] = awkNum(0)
		in.vars["RLENGTH"] = awkNum(-1)
		return awkNum(0), nil
	}
	start := utf8.RuneCountInString(text[:span[0]]) + 1
	in.vars["RSTART"] = awkNum(float64(start))
	in.vars["RLENGTH"] = awkNum(float64(utf8.RuneCountInString(text[span[0]:span[1]])))
	return awkNum(float64(start)), nil
}
