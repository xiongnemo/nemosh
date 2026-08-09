package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// tr is the applet this bundle most needed and did not have, and on Windows it
// is needed twice over: `tr -d '\r'` is the canonical way to strip carriage
// returns from a file that crossed platforms, and nothing else here does it.
//
// The subset is busybox's core: SET1 translated to SET2, -d to delete, -s to
// squeeze runs, and -c to complement the first set. Ranges (`a-z`) and the
// escapes a shell has already had a chance to interpret (`\r`, `\n`, `\t`,
// `\\`) are understood, because a set written inside single quotes reaches the
// applet unexpanded and would otherwise be taken literally.
//
// Character classes (`[:alpha:]`) and equivalence classes are not implemented,
// and are refused by name rather than treated as the literal characters they
// are made of -- which is what would silently happen if the brackets were taken
// at face value.
func newTrApplet() Applet {
	return simpleApplet{name: "tr", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "dsc", "")
		if err != nil {
			return err
		}
		spec, err := trSpec(options, operands)
		if err != nil {
			return err
		}
		return spec.run(ctx, stdout, stdin)
	}}
}

type trTable struct {
	from    []rune
	to      []rune
	delete  bool
	squeeze bool
	// complement inverts the first set, so `tr -cd '0-9'` keeps only digits.
	complement bool
	// squeezeSet is the characters -s collapses runs of, which POSIX takes from
	// the last set given rather than from the first.
	squeezeSet []rune
}

func trSpec(options appletOptions, operands []string) (trTable, error) {
	table := trTable{
		delete:     options.has('d'),
		squeeze:    options.has('s'),
		complement: options.has('c'),
	}
	// POSIX gives tr four forms, and how many operands it wants depends on which
	// one it is in: translating needs two, `-d` alone needs one, `-s` alone needs
	// one, and `-ds` needs both. Exactly one of -d and -s means one operand.
	wanted := 2
	if table.delete != table.squeeze {
		wanted = 1
	}
	if len(operands) < wanted {
		return trTable{}, missingOperand()
	}
	if len(operands) > wanted {
		return trTable{}, fmt.Errorf("extra operand '%s'", operands[wanted])
	}
	from, err := expandTrSet(operands[0])
	if err != nil {
		return trTable{}, err
	}
	table.from = from
	if len(operands) > 1 {
		to, err := expandTrSet(operands[1])
		if err != nil {
			return trTable{}, err
		}
		table.to = to
	}
	// -s squeezes the characters of the *last* set, which is the second when
	// there is one. Squeezing every repeat instead would collapse `aab` under
	// `tr -s ' '`, which asked about blanks and said nothing about letters.
	table.squeezeSet = table.from
	if len(table.to) > 0 {
		table.squeezeSet = table.to
	}
	return table, nil
}

// expandTrSet turns a set as written into the characters it names.
func expandTrSet(set string) ([]rune, error) {
	if strings.Contains(set, "[:") {
		return nil, fmt.Errorf("unsupported tr set: %s; this build implements characters and ranges, not classes", set)
	}
	runes := []rune(unescapeTrSet(set))
	expanded := make([]rune, 0, len(runes))
	for index := 0; index < len(runes); index++ {
		// A range needs a character on each side of the dash; a dash at either
		// end of the set is itself, which is how everyone writes a literal one.
		if index+2 < len(runes) && runes[index+1] == '-' && runes[index+2] >= runes[index] {
			for r := runes[index]; r <= runes[index+2]; r++ {
				expanded = append(expanded, r)
			}
			index += 2
			continue
		}
		expanded = append(expanded, runes[index])
	}
	return expanded, nil
}

// unescapeTrSet reads the backslash escapes tr defines. A set is nearly always
// single-quoted, so the shell hands `\r` over as two characters and tr is the
// one that has to know what they mean.
func unescapeTrSet(set string) string {
	var out strings.Builder
	for index := 0; index < len(set); index++ {
		if set[index] != '\\' || index+1 == len(set) {
			out.WriteByte(set[index])
			continue
		}
		index++
		switch set[index] {
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case '\\':
			out.WriteByte('\\')
		default:
			out.WriteByte(set[index])
		}
	}
	return out.String()
}

// run reads the stream a rune at a time, because a translation is defined on
// characters and a byte-wise pass would cut a multi-byte one in half.
func (t trTable) run(ctx context.Context, stdout io.Writer, stdin io.Reader) error {
	reader := bufio.NewReader(contextReader{ctx: ctx, reader: stdin})
	writer := bufio.NewWriter(stdout)
	previous := rune(-1)
	for {
		r, _, err := reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				return writer.Flush()
			}
			return err
		}
		mapped, keep := t.apply(r)
		if !keep {
			previous = -1
			continue
		}
		if t.squeeze && mapped == previous && t.squeezes(mapped) {
			continue
		}
		previous = mapped
		if _, err := writer.WriteRune(mapped); err != nil {
			return err
		}
	}
}

// apply reports what a rune becomes, and whether it survives at all.
func (t trTable) apply(r rune) (rune, bool) {
	index := indexOfRune(t.from, r)
	matched := index >= 0
	if t.complement {
		matched = !matched
		// A complemented set has no positions, so every match takes the last
		// replacement -- which is what the whole set collapses to.
		index = len(t.to) - 1
	}
	if !matched {
		return r, true
	}
	if t.delete {
		return r, false
	}
	if len(t.to) == 0 {
		return r, true
	}
	// A short second set repeats its last character, which is what makes
	// `tr 'a-z' 'A'` fold a whole alphabet onto one letter.
	if index >= len(t.to) {
		index = len(t.to) - 1
	}
	return t.to[index], true
}

// squeezes reports whether a run of this character is collapsed. Complementing
// inverts the question along with everything else the first set decides.
func (t trTable) squeezes(r rune) bool {
	inSet := indexOfRune(t.squeezeSet, r) >= 0
	if t.complement && len(t.to) == 0 {
		return !inSet
	}
	return inSet
}

func indexOfRune(set []rune, want rune) int {
	for index, r := range set {
		if r == want {
			return index
		}
	}
	return -1
}
