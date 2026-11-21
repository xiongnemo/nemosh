package runtime

import (
	"fmt"
	"slices"
	"strings"
)

// Where a command's name goes, as the alias reader (alias_read.go) follows it through operators,
// lines, reserved words and the headers of for, case and function.

// inPatterns is whether the innermost open case is having its patterns read.
func (s readState) inPatterns() bool {
	return len(s.patterns) > 0 && s.patterns[len(s.patterns)-1]
}

// setPatterns marks whether the innermost open case's patterns are being read, on a copy of the
// list, which a state handed down to an alias's text shares.
func (s readState) setPatterns(reading bool) readState {
	patterns := slices.Clone(s.patterns)
	patterns[len(patterns)-1] = reading
	s.patterns = patterns
	return s
}

// lineEnds is the state at the start of the next line.
func (s readState) lineEnds() readState {
	s.target, s.eligible, s.named, s.header = false, false, false, false
	switch {
	case s.mode == readList || s.mode == readForIn:
		s.mode, s.command = readPlain, true
	case s.mode != readPlain:
	case s.inPatterns():
		s.command = false
	default:
		s.command = true
	}
	return s
}

// after is the state after an operator, rest the text that follows it.
func (s readState) after(operator, rest string) readState {
	named := s.named
	s.named, s.eligible = false, false
	if s.mode == readConditional {
		return s
	}
	switch operator {
	case "<", ">", ">>", "<<", "<<-", "<<<", "<&", ">&", "<>", ">|", "&>", "&>>":
		s.target = true
		return s
	case ";;", ";&", ";;&":
		if len(s.patterns) > 0 {
			s = s.setPatterns(true)
		}
		s.command = false
		return s
	case ")":
		switch {
		case s.header:
			s.header, s.command = false, true
		case s.inPatterns():
			s = s.setPatterns(false)
			s.command = true
		default:
			s.command = false
		}
		return s
	case "(":
		if s.inPatterns() {
			return s
		}
		s.header = named && strings.HasPrefix(strings.TrimLeft(rest, " \t"), ")")
	case "|":
		if s.inPatterns() {
			return s
		}
	}
	if s.mode == readList || s.mode == readForIn {
		s.mode = readPlain
	}
	s.command = true
	return s
}

// headerWord is the state after a word in a compound command's header, or a case pattern.
func (s readState) headerWord(word string) readState {
	switch s.mode {
	case readForName:
		s.mode = readForIn
	case readForIn:
		switch word {
		case "in":
			s.mode = readList
		case "do":
			s.mode, s.command = readPlain, true
		default:
			s.mode = readPlain
		}
	case readCaseWord:
		s.mode = readCaseIn
	case readCaseIn:
		s.mode = readPlain
		if word == "in" {
			s.patterns = append(s.patterns[:len(s.patterns):len(s.patterns)], true)
		}
	case readFunctionName:
		s.mode, s.command = readPlain, true
	case readConditional:
		if word == "]]" {
			s.mode = readPlain
		}
	case readPlain:
		if word == "esac" && len(s.patterns) > 0 {
			s.patterns = s.patterns[:len(s.patterns)-1]
		}
	}
	return s
}

// commandWord is the state after a word written where a command's name goes.
func (s readState) commandWord(word string) readState {
	switch word {
	case "{", "!", "time", "if", "then", "else", "elif", "do", "while", "until":
		return s
	case "}", "fi", "done":
		s.command = false
	case "esac":
		if len(s.patterns) > 0 {
			s.patterns = s.patterns[:len(s.patterns)-1]
		}
		s.command = false
	case "for", "select":
		s.mode, s.command = readForName, false
	case "case":
		s.mode, s.command = readCaseWord, false
	case "function":
		s.mode, s.command = readFunctionName, false
	case "[[":
		s.mode, s.command = readConditional, false
	default:
		if !assignmentWord(word) {
			s.command, s.named = false, true
		}
	}
	return s
}

// assignmentWord is whether a word is an assignment, NAME=, NAME+= or NAME[...]=, after which a
// command's name still goes.
func assignmentWord(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found {
		return false
	}
	name = strings.TrimSuffix(name, "+")
	if open := strings.IndexByte(name, '['); open > 0 && strings.HasSuffix(name, "]") {
		name = name[:open]
	}
	return isValidVariableName(name)
}

// heredocsOf is an alias's text with the here-documents it holds taken out, as the heredoc pass
// took the text's own out before this one reads it (parser.go): their bodies join the text's
// under markers numbered on from theirs, so `alias c='cat <<EOF` with its body and `EOF` in the
// alias is a here-document, as bash 5.3 and busybox read it.
func (a *aliasReader) heredocsOf(value string) string {
	if a.heredocs == nil || !strings.Contains(value, "<<") {
		return value
	}
	cleaned, records, _, _, err := collectHeredocs(normalizeLineEndings(value), true)
	if err != nil || len(records) == 0 {
		return value
	}
	pairs := make([]string, 0, 2*len(records))
	for _, record := range records {
		marker := fmt.Sprintf("__nemosh_heredoc_%d__", len(a.heredocs))
		pairs = append(pairs, record.marker, marker)
		record.marker, record.order = marker, len(a.heredocs)
		a.heredocs[marker] = record
	}
	return strings.NewReplacer(pairs...).Replace(cleaned)
}

// substitutions is a word with the aliases in its command substitutions read, a `$(...)` or a
// backquoted one, each a text of its own; quotes keep what is in them otherwise as it is.
func (a *aliasReader) substitutions(word string) string {
	if !strings.Contains(word, "$(") && !strings.Contains(word, "`") {
		return word
	}
	var out strings.Builder
	double := false
	for index := 0; index < len(word); {
		switch {
		case word[index] == '\\':
			end := min(index+2, len(word))
			out.WriteString(word[index:end])
			index = end
		case word[index] == '\'' && !double:
			end := quoteEnd(word, index+1, '\'')
			out.WriteString(word[index:end])
			index = end
		case word[index] == '"':
			double = !double
			out.WriteByte('"')
			index++
		case strings.HasPrefix(word[index:], "$(("):
			end := readSubstitutionEnd(word, index)
			out.WriteString(word[index:end])
			index = end
		case strings.HasPrefix(word[index:], "$("):
			end := readSubstitutionEnd(word, index)
			out.WriteString("$(")
			a.read(word[index+2:max(end-1, index+2)], nil, readState{command: true}, &out)
			out.WriteString(word[max(end-1, index+2):end])
			index = end
		case word[index] == '`':
			end := quoteEnd(word, index+1, '`')
			out.WriteByte('`')
			a.read(word[index+1:max(end-1, index+1)], nil, readState{command: true}, &out)
			out.WriteString(word[max(end-1, index+1):end])
			index = end
		default:
			out.WriteByte(word[index])
			index++
		}
	}
	return out.String()
}
