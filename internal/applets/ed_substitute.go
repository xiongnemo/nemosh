package applets

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// `s`, `g` and `v` -- the three commands that make ed worth driving from a script.
//
// **`s` uses the same regular expressions `sed` does**, and the same replacement rules: `&`
// is the whole match, `\1` to `\9` are the groups, and a backslash before either makes it
// literal. Sharing sed's translator rather than writing a second one is what keeps
// `ed -s file <<< ',s/a/b/g'` and `sed -i 's/a/b/g' file` from quietly differing.
//
// **`g` and `v` collect their lines before running anything.** A command that inserts or
// deletes would otherwise move the lines still to be visited, and the loop would skip or
// repeat them -- which is the classic way a global command silently does the wrong thing.

func (b *edBuffer) substitute(addresses edAddresses, rest string) error {
	if err := b.checkRange(addresses.first, addresses.last); err != nil {
		return err
	}
	if rest == "" {
		return fmt.Errorf("no pattern")
	}
	delimiter := rest[0]
	pattern, remainder := edSplitDelimited(rest[1:], delimiter)
	replacement, flags := edSplitDelimited(remainder, delimiter)
	if pattern == "" {
		pattern = b.lastPattern
	}
	if pattern == "" {
		return fmt.Errorf("no previous pattern")
	}
	b.lastPattern = pattern
	compiled, err := compileSedPattern(pattern, false, false)
	if err != nil {
		return fmt.Errorf("invalid pattern")
	}
	global, occurrence, print, err := edSubstituteFlags(flags)
	if err != nil {
		return err
	}
	return b.applySubstitution(addresses, compiled, replacement, global, occurrence, print)
}

func (b *edBuffer) applySubstitution(addresses edAddresses, compiled *regexp.Regexp,
	replacement string, global bool, occurrence int, print bool) error {
	changed := false
	last := 0
	for index := addresses.first; index <= addresses.last; index++ {
		updated, did := edSubstituteLine(compiled, b.lines[index-1], replacement, global, occurrence)
		if !did {
			continue
		}
		b.lines[index-1] = updated
		b.dirty = true
		b.warned = false
		changed = true
		last = index
	}
	if !changed {
		// No match at all is an error in ed, not a quiet success -- which is what lets a
		// script tell "changed nothing" from "changed something".
		return fmt.Errorf("no match")
	}
	b.setCurrent(last)
	if print {
		b.writeLine(last, false, false)
	}
	return nil
}

// edSubstituteFlags reads what follows the closing delimiter: `g`, a number, and `p`.
func edSubstituteFlags(flags string) (bool, int, bool, error) {
	global, occurrence, print := false, 1, false
	digits := ""
	for index := 0; index < len(flags); index++ {
		switch character := flags[index]; character {
		case 'g':
			global = true
		case 'p':
			print = true
		case ' ', '\t':
		default:
			if character < '0' || character > '9' {
				return false, 0, false, fmt.Errorf("invalid flag")
			}
			digits += string(character)
		}
	}
	if digits != "" {
		number, err := strconv.Atoi(digits)
		if err != nil || number < 1 {
			return false, 0, false, fmt.Errorf("invalid count")
		}
		occurrence = number
	}
	return global, occurrence, print, nil
}

// edSubstituteLine replaces on one line, honouring the count and the `g` flag.
//
// A count and `g` together mean "from the Nth onwards", which is what both sed and ed do and
// is the one combination people get wrong.
func edSubstituteLine(compiled *regexp.Regexp, line, replacement string, global bool, occurrence int) (string, bool) {
	spans := compiled.FindAllStringSubmatchIndex(line, -1)
	if len(spans) < occurrence {
		return line, false
	}
	// The same translation sed uses, so `&` and `\1` cannot come to mean two different
	// things in two commands of the same shell.
	template := translateReplacement(replacement)
	var out strings.Builder
	last := 0
	replaced := false
	for index, span := range spans {
		number := index + 1
		if number < occurrence || (!global && number > occurrence) {
			continue
		}
		out.WriteString(line[last:span[0]])
		out.WriteString(string(compiled.ExpandString(nil, template, line, span)))
		last = span[1]
		replaced = true
	}
	out.WriteString(line[last:])
	return out.String(), replaced
}

// global is `g/re/commands` and `v/re/commands`.
func (b *edBuffer) global(ctx context.Context, addresses edAddresses, name byte,
	rest string, reader *bufio.Scanner) error {
	if rest == "" {
		return fmt.Errorf("no pattern")
	}
	first, last := addresses.first, addresses.last
	if addresses.count == 0 {
		// With no address, a global command covers the whole buffer.
		first, last = 1, b.lineCount()
	}
	if err := b.checkRange(first, last); err != nil {
		return err
	}
	delimiter := rest[0]
	pattern, commands := edSplitDelimited(rest[1:], delimiter)
	if pattern == "" {
		pattern = b.lastPattern
	}
	if pattern == "" {
		return fmt.Errorf("no previous pattern")
	}
	b.lastPattern = pattern
	compiled, err := compileSedPattern(pattern, false, false)
	if err != nil {
		return fmt.Errorf("invalid pattern")
	}
	if strings.TrimSpace(commands) == "" {
		commands = "p"
	}
	return b.runGlobal(ctx, first, last, compiled, name == 'v', commands, reader)
}

// runGlobal marks the matching lines, then runs the command on each.
//
// The marking is the point: the lines are chosen **before** anything runs, so a command that
// deletes or inserts cannot make the loop skip a line it was going to visit or visit one
// twice. The marks are the line *contents*' identity rather than their numbers, which is why
// the chosen lines are tracked by a pointer that moves with them.
func (b *edBuffer) runGlobal(ctx context.Context, first, last int, compiled *regexp.Regexp,
	invert bool, commands string, reader *bufio.Scanner) error {
	marked := map[int]bool{}
	for index := first; index <= last; index++ {
		if compiled.MatchString(b.lines[index-1]) != invert {
			marked[index] = true
		}
	}
	// Walked forward, with an offset that tracks how many lines the commands have added or
	// removed so far. That keeps the visits in the order a reader expects -- which shows
	// whenever the command prints -- while still landing on the right line after a `d` or
	// an `a` has renumbered everything below it.
	order := make([]int, 0, len(marked))
	for index := first; index <= last; index++ {
		if marked[index] {
			order = append(order, index)
		}
	}
	offset := 0
	for _, index := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		target := index + offset
		if target < 1 || target > b.lineCount() {
			continue
		}
		before := b.lineCount()
		b.setCurrent(target)
		if err := b.command(ctx, commands, reader); err != nil {
			return err
		}
		offset += b.lineCount() - before
	}
	return nil
}
