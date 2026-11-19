package applets

import "strings"

// gensub(regex, replacement, how [, target]) answers target with matches replaced, and leaves
// target as it was: busybox's awk has it as gawk does, and this one called it an undefined
// function. busybox's awk_sub (editors/awk.c:2553-2636) is the rule followed.
//
// how is a number, as busybox's getvar_i reads it, so "g", "G" and any other text that is not a
// number are 0, which is every match; N is the Nth match alone, and an N past the last replaces
// nothing. gawk reads "g" and "G" the same and takes any other as 1, with a warning.
func (in *awkInterp) builtinGensub(node awkBuiltinExpr) (awkValue, error) {
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
	how, err := in.argNumber(node, 2)
	if err != nil {
		return awkValue{}, err
	}
	target := awkExpr(awkFieldExpr{index: awkNumberExpr{value: 0}})
	if len(node.args) > 3 {
		target = node.args[3]
	}
	current, err := in.eval(target)
	if err != nil {
		return awkValue{}, err
	}
	text := current.str(in.convfmt())
	nth := awkToInt64(how)
	var out strings.Builder
	last := 0
	for index, match := range compiled.FindAllStringSubmatchIndex(text, -1) {
		if nth > 0 && int64(index+1) != nth {
			continue
		}
		out.WriteString(text[last:match[0]])
		awkExpandGensub(&out, replacement, text, match)
		last = match[1]
	}
	out.WriteString(text[last:])
	return awkStr(out.String()), nil
}

// awkExpandGensub writes the replacement for one match: `&` and `\0` the matched text, `\1` to
// `\9` the groups, an unmatched or absent one nothing, and a backslash before anything else what
// follows it, so `\&` is a literal `&` and `\\` one backslash. A trailing backslash is itself.
func awkExpandGensub(out *strings.Builder, replacement, text string, match []int) {
	group := func(number int) {
		if 2*number+1 < len(match) && match[2*number] >= 0 {
			out.WriteString(text[match[2*number]:match[2*number+1]])
		}
	}
	for index := 0; index < len(replacement); index++ {
		character := replacement[index]
		switch {
		case character == '\\' && index+1 < len(replacement):
			index++
			if next := replacement[index]; next >= '0' && next <= '9' {
				group(int(next - '0'))
			} else {
				out.WriteByte(next)
			}
		case character == '&':
			group(0)
		default:
			out.WriteByte(character)
		}
	}
}
