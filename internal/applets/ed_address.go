package applets

import (
	"fmt"
	"strings"
)

// ed's addresses: which lines a command applies to.
//
// The forms, all of which combine with `+` and `-`:
//
//	5      .      $      /re/     ?re?     'x
//	1,5    ,      %      ;        .,$
//
// Two rules are worth stating because they are what make a script's `ed` predictable:
//
//   - **A forward search wraps.** `/x/` from the last line looks at the first, which is what
//     lets a script search without knowing where `.` happens to be. busybox does not wrap,
//     which is one of the reasons this follows GNU instead.
//   - **`,` is `1,$` and `;` is `.,$`.** The second is the useful one and the one people
//     forget: `;` starts from where the last command left off.

// edAddresses is what a command was given, and how many addresses were written.
type edAddresses struct {
	first, last int
	count       int
}

// parseAddresses reads the address part and answers the rest of the command line.
func (b *edBuffer) parseAddresses(text string) (edAddresses, string, error) {
	addresses := edAddresses{first: b.current, last: b.current}
	rest := strings.TrimLeft(text, " \t")
	switch {
	case strings.HasPrefix(rest, ","):
		return edAddresses{first: 1, last: b.lineCount(), count: 2}, rest[1:], nil
	case strings.HasPrefix(rest, "%"):
		return edAddresses{first: 1, last: b.lineCount(), count: 2}, rest[1:], nil
	case strings.HasPrefix(rest, ";"):
		return edAddresses{first: b.current, last: b.lineCount(), count: 2}, rest[1:], nil
	}
	first, rest, found, err := b.parseOneAddress(rest)
	if err != nil {
		return addresses, rest, err
	}
	if !found {
		return addresses, rest, nil
	}
	addresses.first, addresses.last, addresses.count = first, first, 1
	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, ",") && !strings.HasPrefix(rest, ";") {
		return addresses, rest, nil
	}
	// The second address of a range. A `;` also moves `.` to the first, which is what
	// makes `/a/;/b/p` search for `b` *after* `a` rather than from where it started.
	separator := rest[0]
	rest = rest[1:]
	if separator == ';' {
		b.setCurrent(first)
	}
	last, rest, found, err := b.parseOneAddress(rest)
	if err != nil {
		return addresses, rest, err
	}
	if !found {
		last = b.lineCount()
	}
	addresses.last, addresses.count = last, 2
	return addresses, rest, nil
}

// parseOneAddress reads a single address, with any `+N` or `-N` after it.
func (b *edBuffer) parseOneAddress(text string) (int, string, bool, error) {
	rest := strings.TrimLeft(text, " \t")
	line, rest, found, err := b.parseAddressBase(rest)
	if err != nil {
		return 0, rest, false, err
	}
	for {
		rest = strings.TrimLeft(rest, " \t")
		if len(rest) == 0 || (rest[0] != '+' && rest[0] != '-') {
			return line, rest, found, nil
		}
		sign := 1
		if rest[0] == '-' {
			sign = -1
		}
		rest = rest[1:]
		// A bare `+` or `-` means one line, which is what makes `-` on its own step back.
		step := 1
		digits := 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		if digits > 0 {
			step = 0
			for _, digit := range rest[:digits] {
				step = step*10 + int(digit-'0')
			}
			rest = rest[digits:]
		}
		if !found {
			line, found = b.current, true
		}
		line += sign * step
	}
}

func (b *edBuffer) parseAddressBase(rest string) (int, string, bool, error) {
	if len(rest) == 0 {
		return 0, rest, false, nil
	}
	switch rest[0] {
	case '.':
		return b.current, rest[1:], true, nil
	case '$':
		return b.lineCount(), rest[1:], true, nil
	case '\'':
		if len(rest) < 2 {
			return 0, rest, false, fmt.Errorf("invalid mark")
		}
		line, marked := b.marks[rest[1]]
		if !marked {
			return 0, rest, false, fmt.Errorf("invalid mark")
		}
		return line, rest[2:], true, nil
	case '/', '?':
		return b.parseSearch(rest)
	}
	if rest[0] >= '0' && rest[0] <= '9' {
		line, digits := 0, 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			line = line*10 + int(rest[digits]-'0')
			digits++
		}
		return line, rest[digits:], true, nil
	}
	return 0, rest, false, nil
}

// parseSearch reads `/re/` or `?re?` and looks for it.
func (b *edBuffer) parseSearch(rest string) (int, string, bool, error) {
	delimiter := rest[0]
	pattern, remainder := edSplitDelimited(rest[1:], delimiter)
	if pattern == "" {
		// An empty pattern repeats the last one, which is what `//` is for.
		pattern = b.lastPattern
	}
	if pattern == "" {
		return 0, remainder, false, fmt.Errorf("no previous pattern")
	}
	b.lastPattern = pattern
	compiled, err := compileAwkRegex(pattern)
	if err != nil {
		return 0, remainder, false, fmt.Errorf("invalid pattern")
	}
	total := b.lineCount()
	if total == 0 {
		return 0, remainder, false, fmt.Errorf("no match")
	}
	// Wrapping, in both directions: the search visits every line once, starting from the
	// one after (or before) the current.
	for step := 1; step <= total; step++ {
		index := b.current + step
		if delimiter == '?' {
			index = b.current - step
		}
		index = ((index-1)%total + total) % total
		if compiled.MatchString(b.lines[index]) {
			return index + 1, remainder, true, nil
		}
	}
	return 0, remainder, false, fmt.Errorf("no match")
}

// edSplitDelimited takes the text up to an unescaped delimiter.
func edSplitDelimited(text string, delimiter byte) (string, string) {
	var out strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] == '\\' && index+1 < len(text) {
			// A delimiter escaped with a backslash is part of the pattern, and the
			// backslash goes with it so that `\.` stays a literal dot.
			if text[index+1] == delimiter {
				out.WriteByte(delimiter)
				index++
				continue
			}
			out.WriteByte(text[index])
			out.WriteByte(text[index+1])
			index++
			continue
		}
		if text[index] == delimiter {
			return out.String(), text[index+1:]
		}
		out.WriteByte(text[index])
	}
	return out.String(), ""
}
