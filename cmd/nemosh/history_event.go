package main

import (
	"errors"
	"math"
	"strings"
)

// One history expansion, bash's history_expand_internal: the event a `!` names, the words a
// designator takes from it, and what its modifiers make of them.

// lastWord is `$` as a word designator's bound: the event's last word.
const lastWord = math.MaxInt32

// historyError is hist_error: the text of the expansion that failed, then why.
func historyError(text string, start, end int, why string) error {
	return errors.New(text[start:min(end, len(text))] + ": " + why)
}

// expandOne is the `!` at text[start] and what follows it, answered as its replacement, the
// index of the last byte it took, and whether a :p was among its modifiers. current is the
// line as expanded so far, which `!#` is.
func (x *historyExpander) expandOne(text string, start int, dquote bool, current string, entries []string) (string, int, bool, error) {
	at := func(k int) byte {
		if k < len(text) {
			return text[k]
		}
		return 0
	}
	i := start
	var event string
	found := true
	switch {
	case byteIn(":$*%^", at(start+1)):
		// A word designator with no event is the previous line's.
		i++
		event, found = historyEntry(entries, -1)
	case at(start+1) == '#':
		i += 2
		event = current
	default:
		var quote byte
		if dquote {
			quote = '"'
		}
		event, found, i = x.event(text, start, quote, entries)
	}
	if !found {
		return "", 0, false, historyError(text, start, i, "event not found")
	}
	wordsAt := i
	words, designated, valid := x.wordDesignator(text, event, &i)
	if !valid {
		return "", 0, false, historyError(text, wordsAt, i, "bad word specifier")
	}
	value := event
	if designated {
		value = words
	}
	var quoting byte
	global, byWords, printOnly := 0, false, false
	modifiersAt := i
	for at(i) == ':' {
		c := at(i + 1)
		switch c {
		case 'g', 'a':
			global = 1
			i++
			c = at(i + 1)
		case 'G':
			byWords = true
			i++
			c = at(i + 1)
		}
		switch c {
		case 'q', 'x':
			quoting = c
		case 'p':
			printOnly = true
		case 't':
			if slash := strings.LastIndexByte(value, '/'); slash >= 0 {
				value = value[slash+1:]
			}
		case 'h':
			if slash := strings.LastIndexByte(value, '/'); slash >= 0 {
				value = value[:slash]
			}
		case 'r':
			if dot := strings.LastIndexByte(value, '.'); dot >= 0 {
				value = value[:dot]
			}
		case 'e':
			if dot := strings.LastIndexByte(value, '.'); dot >= 0 {
				value = value[dot:]
			}
		case 's', '&':
			if c == 's' {
				if i+2 >= len(text) {
					// No delimiter: the :s is taken, and does nothing.
					break
				}
				x.readSubstitution(text, &i)
			} else {
				i += 2
			}
			if x.lhs == "" {
				return "", 0, false, historyError(text, modifiersAt, i, "no previous substitution")
			}
			substituted, ok := x.substitute(value, &global, byWords)
			if !ok {
				return "", 0, false, historyError(text, modifiersAt, i, "substitution failed")
			}
			value = substituted
			continue
		default:
			return "", 0, false, historyError(text, i+1, i+2, "unrecognized history modifier")
		}
		i += 2
	}
	switch quoting {
	case 'q':
		value = shSingleQuote(value)
	case 'x':
		value = quoteBreaks(value)
	}
	return value, i - 1, printOnly, nil
}

// event is get_history_event: the line the `!` at text[i] names -- `!!`, `!n`, `!-n`,
// `!string`, the newest line that begins with it, or `!?string?`, the newest that holds it --
// and the index after its specification. quote, the double quote the `!` is inside, ends a
// string too.
func (x *historyExpander) event(text string, i int, quote byte, entries []string) (string, bool, int) {
	at := func(k int) byte {
		if k < len(text) {
			return text[k]
		}
		return 0
	}
	i++
	if at(i) == '!' {
		line, ok := historyEntry(entries, -1)
		return line, ok, i + 1
	}
	negative := at(i) == '-' && isDigit(at(i+1))
	if negative {
		i++
	}
	if isDigit(at(i)) {
		number := 0
		for ; isDigit(at(i)); i++ {
			number = min(number*10+int(at(i)-'0'), math.MaxInt32)
		}
		if negative {
			number = len(entries) + 1 - number
		}
		if number <= 0 {
			return "", false, i
		}
		line, ok := historyEntry(entries, number)
		return line, ok, i
	}
	substring := at(i) == '?'
	if substring {
		i++
	}
	begin := i
	for ; i < len(text); i++ {
		c := text[i]
		if c == '\n' || substring && c == '?' {
			break
		}
		if !substring && (c == ' ' || c == '\t' || c == ':' || i > begin && c == '-' ||
			c != '-' && byteIn(historyEventDelimiters, c) || byteIn(historySearchDelimiters, c) || c == quote) {
			break
		}
	}
	needle := text[begin:i]
	if substring && at(i) == '?' {
		i++
	}
	if needle == "" && substring {
		needle = x.search
	}
	if needle == "" {
		return "", false, i
	}
	for index := len(entries) - 1; index >= 0; index-- {
		line := entries[index]
		if !substring {
			if strings.HasPrefix(line, needle) {
				return line, true, i
			}
			continue
		}
		if found := strings.LastIndex(line, needle); found >= 0 {
			x.search, x.match = needle, historyFindWord(line, found)
			return line, true, i
		}
	}
	return "", false, i
}
