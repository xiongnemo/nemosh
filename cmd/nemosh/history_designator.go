package main

import (
	"math"
	"strings"
)

// A word designator's words, bash's get_history_word_specifier and history_arg_extract.

// wordDesignator is get_history_word_specifier: the words of event the designator at
// text[*index] takes -- `0`, `n`, `^`, `$`, `%`, `x-y`, `x*`, `x-` and `*` -- joined by blanks;
// whether there was one; and false for one naming words the event has not. Without a `:`
// only `^`, `$`, `*`, `-` and `%` begin one.
func (x *historyExpander) wordDesignator(text, event string, index *int) (string, bool, bool) {
	at := func(k int) byte {
		if k < len(text) {
			return text[k]
		}
		return 0
	}
	i := *index
	expecting := at(i) == ':'
	if expecting {
		i++
	}
	switch at(i) {
	case '%':
		*index = i + 1
		return x.match, true, true
	case '*':
		*index = i + 1
		words, _ := historyArgs(1, lastWord, event)
		return words, true, true
	case '$':
		*index = i + 1
		words, ok := historyArgs(lastWord, lastWord, event)
		return words, ok, true
	}
	first, last := 0, 0
	switch c := at(i); {
	case c == '-':
	case c == '^':
		first = 1
		i++
	case isDigit(c) && expecting:
		first, i = historyNumberAt(text, i)
	default:
		return "", false, true
	}
	switch c := at(i); {
	case c == '^':
		last = 1
		i++
	case c == '*':
		last = lastWord
		i++
	case c != '-':
		last = first
	default:
		i++
		switch c := at(i); {
		case isDigit(c):
			last, i = historyNumberAt(text, i)
		case c == '$':
			last = lastWord
			i++
		case c == '^':
			last = 1
			i++
		default:
			// `x-` is x to the word before the last.
			last = -1
		}
	}
	*index = i
	if last >= first || last < 0 {
		if words, ok := historyArgs(first, last, event); ok {
			return words, true, true
		}
	}
	return "", true, false
}

func historyNumberAt(text string, i int) (int, int) {
	number := 0
	for ; i < len(text) && isDigit(text[i]); i++ {
		number = min(number*10+int(text[i]-'0'), math.MaxInt32-1)
	}
	return number, i
}

// historyArgs is history_arg_extract: words first to last of line, last -1 being the one
// before the last, joined by blanks; false where the line has not got them.
func historyArgs(first, last int, line string) (string, bool) {
	words := historyTokenize(line)
	count := len(words)
	if count == 0 {
		return "", false
	}
	if last < 0 {
		last = count + last - 1
	}
	if last == lastWord {
		last = count - 1
	}
	if first == lastWord {
		first = count - 1
	}
	last++
	if first >= count || last > count || first < 0 || last < 0 || first > last {
		return "", false
	}
	return strings.Join(words[first:last], " "), true
}
