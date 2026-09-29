package applets

import (
	"errors"
	"strings"
)

// sortFlag is one of sort's letters, at the bit busybox's sort.c gives it: the orderings
// -n -g -h -M -V, then -u -c -s -z, which only the whole command takes, then -b -r -d -f -i,
// which a key takes too.
type sortFlag uint32

const (
	sortNumeric sortFlag = 1 << iota
	sortGeneral
	sortHuman
	sortMonth
	sortVersion
	sortUnique
	sortCheck
	sortStable
	sortZero
	sortBlanks
	sortReverse
	sortDictionary
	sortFold
	sortPrintable
	// sortTrailingBlanks strips a key's trailing blanks: b after a key's comma, and -b outside one.
	sortTrailingBlanks
)

// sortFlagLetters are the letters of the flags above, in their order.
const sortFlagLetters = "nghMVucszbrdfi"

const (
	sortOrderings  = sortNumeric | sortGeneral | sortHuman | sortMonth | sortVersion
	sortKeyLetters = sortNumeric | sortGeneral | sortHuman | sortMonth | sortBlanks | sortReverse | sortDictionary | sortFold | sortPrintable
	sortTrimming   = sortBlanks | sortTrailingBlanks | sortDictionary | sortFold | sortPrintable
)

// sortOptionString is busybox's OPT_STR, which a key's letters are looked up in: one that is not
// there is an unknown key option, and one that is there but does not order a key, -k2u, is an
// unknown sort type.
const sortOptionString = "nghMVucszbrdfimS:T:o:k:*t:"

type sortSpec struct {
	flags     sortFlag
	keys      []sortKey
	separator byte
	output    string
}

// sortKey is -k's POS1[,POS2], each a field and a character, one-based. A POS1 field of 0 starts
// at the line's end and a POS2 field of 0 ends there; a character of 0 is the field's first for
// POS1 and its last for POS2.
type sortKey struct {
	field, char [2]int
	flags       sortFlag
}

// parseSortKey reads FIELD[.CHAR][LETTERS][,FIELD[.CHAR][LETTERS]], as busybox's sort_main does.
// An empty -k is a key that is empty for every line, which busybox takes too.
func parseSortKey(text string) (sortKey, error) {
	var key sortKey
	position := 0
	for index := 0; index < len(text); {
		field, next, err := sortKeyNumber(text, index)
		if err != nil {
			return key, err
		}
		key.field[position], index = field, next
		if index < len(text) && text[index] == '.' {
			if key.char[position], index, err = sortKeyNumber(text, index+1); err != nil {
				return key, err
			}
		}
		for index < len(text) {
			letter := text[index]
			index++
			if letter == ',' && position == 0 {
				position = 1
				break
			}
			if strings.IndexByte(sortOptionString, letter) < 0 {
				return key, errors.New("unknown key option")
			}
			flag := sortFlag(0)
			if bit := strings.IndexByte(sortFlagLetters, letter); bit >= 0 {
				flag = 1 << bit
			}
			if flag&sortKeyLetters == 0 {
				return key, errors.New("unknown sort type")
			}
			// b after the comma strips the key's trailing blanks.
			if position == 1 && flag == sortBlanks {
				flag = sortTrailingBlanks
			}
			key.flags |= flag
		}
	}
	return key, nil
}

// sortKeyNumber is busybox's str2u: digits, from 1.
func sortKeyNumber(text string, index int) (int, int, error) {
	start, value := index, 0
	for index < len(text) && isASCIIDigit(text[index]) && value <= 1<<31-1 {
		value = value*10 + int(text[index]-'0')
		index++
	}
	if index == start || value == 0 || value > 1<<31-1 {
		return 0, index, errors.New("bad field specification")
	}
	return value, index, nil
}

// keyOf is the part of line that key compares, as busybox's get_key cuts it: from the start of
// POS1's field -- its leading blanks with it, unless b -- to the end of POS2's, then -d, -i and -f.
func (spec sortSpec) keyOf(line string, key sortKey, flags sortFlag) string {
	if key.field == [2]int{1, 0} && key.char == [2]int{} && flags&sortTrimming == 0 {
		return line
	}
	start := spec.fieldStart(line, key.field[0])
	end := spec.fieldEnd(line, key.field[1])
	if flags&sortBlanks != 0 {
		for start < len(line) && isCSpace(line[start]) {
			start++
		}
	}
	if flags&sortTrailingBlanks != 0 {
		for end > start && isCSpace(line[end-1]) {
			end--
		}
	}
	// POS2's character is counted from the start of its field, past its blanks under b, as POSIX
	// has it. busybox counts it from the start of the line, which is the same for the first field
	// and no key at all for the rest: -k2.1,2.1 compared nothing.
	if key.char[1] > 0 {
		from := spec.fieldStart(line, key.field[1])
		for flags&sortTrailingBlanks != 0 && from < len(line) && isCSpace(line[from]) {
			from++
		}
		end = min(from+key.char[1], len(line))
	}
	if key.char[0] > 0 {
		start = min(start+key.char[0]-1, len(line))
	}
	end = max(end, start)
	text := line[start:end]
	if flags&(sortDictionary|sortPrintable|sortFold) == 0 {
		return text
	}
	kept := make([]byte, 0, len(text))
	for index := range len(text) {
		c := text[index]
		switch {
		case flags&sortDictionary != 0 && !isCSpace(c) && !isASCIIAlnum(c):
		case flags&sortPrintable != 0 && (c < 0x20 || c > 0x7e):
		case flags&sortFold != 0 && 'a' <= c && c <= 'z':
			kept = append(kept, c-'a'+'A')
		default:
			kept = append(kept, c)
		}
	}
	return string(kept)
}

// fieldStart is where field starts: after the fields before it and, with -t, the separator that
// ends each. Without -t a field is its leading blanks and what follows up to the next blank.
func (spec sortSpec) fieldStart(line string, field int) int {
	if field == 0 {
		return len(line)
	}
	position := 0
	for range field - 1 {
		position, _ = spec.skipField(line, position)
	}
	return position
}

// fieldEnd is where field ends, before the separator that follows it.
func (spec sortSpec) fieldEnd(line string, field int) int {
	if field == 0 {
		return len(line)
	}
	position, separated := 0, false
	for range field {
		position, separated = spec.skipField(line, position)
	}
	if separated {
		position--
	}
	return position
}

// skipField passes one field from position, and with -t the separator after it, saying whether
// it passed one.
func (spec sortSpec) skipField(line string, position int) (int, bool) {
	if spec.separator != 0 {
		if index := strings.IndexByte(line[position:], spec.separator); index >= 0 {
			return position + index + 1, true
		}
		return len(line), false
	}
	for position < len(line) && isCSpace(line[position]) {
		position++
	}
	for position < len(line) && !isCSpace(line[position]) {
		position++
	}
	return position, false
}

// compare is busybox's compare_keys: each key in turn, as the key's letters or else the ones
// outside a key say; then, unless -s or tieBreak is off, the lines whole, byte by byte, as the
// options outside a key say. -r reverses whichever decided, except that -s keeps tied lines in
// the order they came.
func (spec sortSpec) compare(left, right string, tieBreak bool) int {
	flags, result := spec.flags, 0
	for _, key := range spec.keys {
		if flags = key.flags; flags == 0 {
			flags = spec.flags
		}
		if result = compareSortKey(spec.keyOf(left, key, flags), spec.keyOf(right, key, flags), flags); result != 0 {
			break
		}
	}
	if result == 0 && tieBreak {
		if spec.flags&sortStable != 0 {
			return 0
		}
		flags, result = spec.flags, strings.Compare(left, right)
	}
	if flags&sortReverse != 0 {
		return -result
	}
	return result
}

func compareSortKey(left, right string, flags sortFlag) int {
	switch flags & sortOrderings {
	case sortNumeric:
		leftValue, _ := cNumber(left, false)
		rightValue, _ := cNumber(right, false)
		return compareFloat(leftValue, rightValue)
	case sortGeneral, sortHuman:
		return compareGeneral(left, right, flags&sortHuman != 0)
	case sortMonth:
		leftMonth, leftOK := monthOf(left)
		rightMonth, rightOK := monthOf(right)
		switch {
		case !leftOK && !rightOK:
			return 0
		case !leftOK:
			return -1
		case !rightOK:
			return 1
		}
		return leftMonth - rightMonth
	case sortVersion:
		return strverscmp(left, right)
	}
	return strings.Compare(left, right)
}

func isCSpace(c byte) bool {
	return c == ' ' || '\t' <= c && c <= '\r'
}

func isASCIIDigit(c byte) bool { return '0' <= c && c <= '9' }

func isASCIIAlnum(c byte) bool {
	return isASCIIDigit(c) || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}
