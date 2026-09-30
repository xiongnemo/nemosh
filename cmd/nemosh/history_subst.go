package main

import "strings"

// :s and :&, bash's substitution modifiers.

// readSubstitution reads the :s at text[*i]: lhs and rhs end at the delimiter, the byte
// after the s, or at the line's end, and a backslash escapes the delimiter in either. An empty
// lhs is the last one, or else the last `!?string?` search's string. In rhs an & is lhs, and
// a backslash before an & makes it one.
func (x *historyExpander) readSubstitution(text string, i *int) {
	delimiter := text[*i+2]
	if delimiter >= 0x80 {
		// A delimiter of more than one byte is none, as in bash: the rest of the line is lhs.
		delimiter = 0
	}
	*i += 3
	if lhs, ok := substPattern(text, i, delimiter, false); ok {
		x.lhs = lhs
	} else if x.lhs == "" {
		x.lhs = x.search
	}
	x.rhs, _ = substPattern(text, i, delimiter, true)
	if x.lhs != "" && strings.Contains(x.rhs, "&") {
		var rhs strings.Builder
		for k := 0; k < len(x.rhs); k++ {
			switch {
			case x.rhs[k] == '&':
				rhs.WriteString(x.lhs)
			case x.rhs[k] == '\\' && k+1 < len(x.rhs) && x.rhs[k+1] == '&':
				k++
				rhs.WriteByte('&')
			default:
				rhs.WriteByte(x.rhs[k])
			}
		}
		x.rhs = rhs.String()
	}
}

// substPattern is get_subst_pattern: the pattern at text[*i] up to delimiter, with *i left
// past the delimiter; false for an empty lhs.
func substPattern(text string, i *int, delimiter byte, rhs bool) (string, bool) {
	start, end := *i, *i
	for end < len(text) && text[end] != delimiter {
		if text[end] == '\\' && end+1 < len(text) && text[end+1] == delimiter {
			end++
		}
		end++
	}
	var pattern strings.Builder
	for k := start; k < end; k++ {
		if text[k] == '\\' && k+1 < len(text) && text[k+1] == delimiter {
			k++
		}
		pattern.WriteByte(text[k])
	}
	*i = end
	if end < len(text) {
		*i = end + 1
	}
	return pattern.String(), end > start || rhs
}

// substitute is :s and :&: lhs replaced by rhs in value, the first time, or every time after
// a g, or the first time in each word after a G. global counts the replacements a g made, and
// is reset once they have been made, as bash's substitute_globally is.
func (x *historyExpander) substitute(value string, global *int, byWords bool) (string, bool) {
	lhs, rhs := x.lhs, x.rhs
	if len(lhs) > len(value) {
		return "", false
	}
	failed := true
	wordEnd := 0
	for at := 0; at+len(lhs) <= len(value); at++ {
		if byWords && at > wordEnd {
			for at < len(value) && isHistoryBlank(value[at]) {
				at++
			}
			wordEnd = historyWordEnd(value, at)
		}
		if !strings.HasPrefix(value[at:], lhs) {
			continue
		}
		value = value[:at] + rhs + value[at+len(lhs):]
		failed = false
		if *global > 0 {
			at += len(rhs) - 1
			*global++
			continue
		}
		if byWords {
			at = wordEnd
			continue
		}
		break
	}
	if *global > 1 {
		*global = 0
		return value, true
	}
	return value, !failed
}
