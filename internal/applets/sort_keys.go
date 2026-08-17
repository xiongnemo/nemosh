package applets

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// sort's key and comparison options, measured from GNU.
//
//	$ printf 'b 2\na 3\nc 1\n' | sort -k2       ->  c 1 / b 2 / a 3
//	$ printf 'B\nb\na\n'       | sort -uf       ->  a / B
//	$ printf '3:a\n1:b\n'      | sort -t: -k1 -n ->  1:b / 3:a
//
// -k is the one that was missed most, and the one whose absence is least visible:
// without it `sort -k2` was refused outright, so at least nothing silently sorted
// by the wrong column.

// sortFields adds the options that decide *what* is compared, on top of the
// numeric and reverse flags sort already had.
type sortFields struct {
	unique       bool
	foldCase     bool
	ignoreBlanks bool
	// separator is -t. Empty means the default: runs of blanks, with the field
	// starting at the first non-blank -- which is not the same as splitting on a
	// single space, and is why `sort -k2` works on aligned columns.
	separator string
	// keyFrom and keyTo are -k's one-based field range. Zero means the whole
	// line.
	keyFrom int
	keyTo   int
}

// parseSortKey reads -k's argument, which GNU spells `F[.C][OPTS][,F[.C][OPTS]]`.
// Only the field numbers are honoured here; a character offset or a per-key
// modifier is refused rather than ignored, because a key that means something
// slightly different from what was asked is worse than no key at all.
func parseSortKey(value string, into *sortFields) error {
	from, to, hasTo := strings.Cut(value, ",")
	start, err := strconv.Atoi(from)
	if err != nil || start < 1 {
		return fmt.Errorf("sort: invalid key: %s", value)
	}
	into.keyFrom = start
	into.keyTo = start
	if !hasTo {
		// A key with no end runs to the end of the line, which is GNU's rule and
		// the reason `sort -k2` on three-field lines compares fields two and
		// three together.
		into.keyTo = 0
		return nil
	}
	end, err := strconv.Atoi(to)
	if err != nil || end < start {
		return fmt.Errorf("sort: invalid key: %s", value)
	}
	into.keyTo = end
	return nil
}

// sortKeyOf extracts the part of the line the comparison should see.
func (f sortFields) sortKeyOf(line string) string {
	if f.keyFrom == 0 {
		return f.prepare(line)
	}
	fields := f.splitFields(line)
	if f.keyFrom > len(fields) {
		// A line with too few fields has an empty key, which sorts first. GNU
		// does the same rather than treating the line as absent.
		return ""
	}
	end := len(fields)
	if f.keyTo > 0 && f.keyTo < end {
		end = f.keyTo
	}
	return f.prepare(strings.Join(fields[f.keyFrom-1:end], " "))
}

// splitFields is where -t decides everything.
//
// Without -t, GNU's field separator is the *transition* from blank to non-blank,
// so a run of spaces is one separator and leading blanks belong to the first
// field. Splitting on a single space would make `a   b` four fields, and then
// `-k2` would compare the empty string.
func (f sortFields) splitFields(line string) []string {
	if f.separator == "" {
		return strings.FieldsFunc(line, unicode.IsSpace)
	}
	return strings.Split(line, f.separator)
}

// prepare applies the comparison-time options: -b drops leading blanks and -f
// folds case.
func (f sortFields) prepare(text string) string {
	if f.ignoreBlanks {
		text = strings.TrimLeft(text, " \t")
	}
	if f.foldCase {
		text = strings.ToUpper(text)
	}
	return text
}

// uniqueSorted removes adjacent duplicates *by the comparison key*, which is what
// makes `sort -uf` on `B b a` answer `a B` rather than all three: -u means unique
// according to the sort, not unique as text.
func uniqueSorted(lines []string, fields sortFields, numeric bool) []string {
	if len(lines) == 0 {
		return lines
	}
	kept := lines[:1]
	for _, line := range lines[1:] {
		previous := kept[len(kept)-1]
		if compareSortKeys(previous, line, fields, numeric) == 0 {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

func compareSortKeys(left, right string, fields sortFields, numeric bool) int {
	leftKey := fields.sortKeyOf(left)
	rightKey := fields.sortKeyOf(right)
	if numeric {
		leftNumber := sortNumericPrefix(leftKey)
		rightNumber := sortNumericPrefix(rightKey)
		switch {
		case leftNumber < rightNumber:
			return -1
		case leftNumber > rightNumber:
			return 1
		}
		// Equal numbers fall through to a text comparison, which is what keeps
		// the order stable and total rather than leaving equal-looking lines in
		// input order only by luck.
	}
	return strings.Compare(leftKey, rightKey)
}

// sortValueOption handles -k and -t, reporting how many arguments it consumed and
// the letters that preceded it in the same word.
func sortValueOption(args []string, index int, fields *sortFields) (string, int, error) {
	arg := args[index]
	position := strings.IndexAny(arg, "kt")
	if position < 1 {
		return "", 0, nil
	}
	letter := arg[position]
	before := arg[1:position]
	value := arg[position+1:]
	consumed := 1
	if value == "" {
		if index+1 >= len(args) {
			return "", 0, fmt.Errorf("sort: option requires an argument -- %c", letter)
		}
		value = args[index+1]
		consumed = 2
	}
	if letter == 't' {
		if value == "" {
			return "", 0, fmt.Errorf("sort: empty tab")
		}
		fields.separator = value
		return before, consumed, nil
	}
	if err := parseSortKey(value, fields); err != nil {
		return "", 0, err
	}
	return before, consumed, nil
}
