package applets

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// factor, fold, tsort and strings: four small tools in one file because each is a
// handful of lines and they share nothing but the operand seam.
//
// Every output shape was measured against busybox-w32 v1.38.0 on 2026-08-22.

// newFactorApplet prints each number's prime factorisation.
//
//	12: 2 2 3
//	1:
//
// One is not prime and has no factors, so its line is a bare `1:` -- measured,
// and the case a naive loop gets wrong.
func newFactorApplet() Applet {
	return simpleApplet{name: "factor", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		_, operands, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		if len(operands) > 0 {
			for _, operand := range operands {
				if err := writeFactors(stdout, operand); err != nil {
					return err
				}
			}
			return nil
		}
		// With no operands the numbers come from stdin, any number per line.
		return eachTextInput(ctx, nil, stdin, func(reader io.Reader) error {
			return eachLine(reader, func(line, _ string) error {
				for _, field := range strings.Fields(line) {
					if err := writeFactors(stdout, field); err != nil {
						return err
					}
				}
				return nil
			})
		})
	}}
}

func writeFactors(stdout io.Writer, text string) error {
	value, err := strconv.ParseUint(strings.TrimSpace(text), 10, 64)
	if err != nil {
		return fmt.Errorf("invalid number '%s'", text)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%d:", value)
	remaining := value
	for remaining > 1 && remaining%2 == 0 {
		out.WriteString(" 2")
		remaining /= 2
	}
	// Odd divisors only, bounded by the square root of what is *left* rather
	// than of the original -- which is what stops a large prime factor costing a
	// scan all the way up to itself.
	for divisor := uint64(3); divisor*divisor <= remaining; divisor += 2 {
		for remaining%divisor == 0 {
			fmt.Fprintf(&out, " %d", divisor)
			remaining /= divisor
		}
	}
	if remaining > 1 {
		fmt.Fprintf(&out, " %d", remaining)
	}
	_, err = fmt.Fprintln(stdout, out.String())
	return err
}

// newFoldApplet wraps long lines. -w sets the width, default 80; -s prefers to
// break at a space.
func newFoldApplet() Applet {
	return simpleApplet{name: "fold", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "bs", "w")
		if err != nil {
			return err
		}
		width := 80
		if options.has('w') {
			parsed, err := strconv.Atoi(options.value('w'))
			if err != nil || parsed <= 0 {
				return fmt.Errorf("illegal width value '%s'", options.value('w'))
			}
			width = parsed
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			// The breaks fold *inserts* are newlines; the ending the input had goes
			// on the last piece only. Measured against busybox, which is the only
			// way to know: folding a CRLF line at width three answers three pieces,
			// the first two ended by a bare newline and the last keeping the CRLF --
			// so the file keeps its endings at the real ends of lines and gets plain
			// newlines only where a line was cut.
			return eachLine(reader, func(line, ending string) error {
				pieces := foldLine(line, width, options.has('s'))
				for index, piece := range pieces {
					terminator := "\n"
					if index == len(pieces)-1 {
						terminator = ending
					}
					if _, err := io.WriteString(stdout, piece+terminator); err != nil {
						return err
					}
				}
				return nil
			})
		})
	}}
}

// foldLine cuts one line into pieces of at most width runes.
//
// Runes, not bytes, so a CJK line is never cut through the middle of a character
// -- the same reason `rev` decodes rather than reversing bytes. An empty line
// yields one empty piece rather than none, because dropping it would lose a line.
func foldLine(line string, width int, atSpaces bool) []string {
	runes := []rune(line)
	if len(runes) == 0 {
		return []string{""}
	}
	var pieces []string
	for len(runes) > width {
		cut := width
		if atSpaces {
			// The last space inside the limit, if there is one. Without one the
			// cut stays hard: -s prefers a space, it does not require one.
			for index := width; index > 0; index-- {
				if runes[index-1] == ' ' {
					cut = index
					break
				}
			}
		}
		pieces = append(pieces, string(runes[:cut]))
		runes = runes[cut:]
	}
	return append(pieces, string(runes))
}

// newTsortApplet topologically sorts `before after` pairs.
func newTsortApplet() Applet {
	return simpleApplet{name: "tsort", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		_, paths, err := parseAppletOptions(args, "", "")
		if err != nil {
			return err
		}
		graph := &tsortGraph{seen: map[string]bool{}, after: map[string][]string{}}
		collect := func(reader io.Reader) error {
			return eachLine(reader, func(line, _ string) error {
				graph.addPairs(strings.Fields(line))
				return nil
			})
		}
		if err := eachTextInput(ctx, paths, stdin, collect); err != nil {
			return err
		}
		return graph.write(stdout)
	}}
}

// tsortGraph keeps the order names were first seen, which is what makes the
// output stable: items with no dependency between them come out in input order,
// so two runs over one file agree and a diff between them means something.
type tsortGraph struct {
	order []string
	seen  map[string]bool
	after map[string][]string
}

func (g *tsortGraph) note(name string) {
	if !g.seen[name] {
		g.seen[name] = true
		g.order = append(g.order, name)
	}
}

func (g *tsortGraph) addPairs(fields []string) {
	for index := 0; index+1 < len(fields); index += 2 {
		g.note(fields[index])
		g.note(fields[index+1])
		if fields[index] != fields[index+1] {
			g.after[fields[index]] = append(g.after[fields[index]], fields[index+1])
		}
	}
	// An odd trailing field still names an item, which is how a standalone node
	// with no edges reaches the output.
	if len(fields)%2 == 1 {
		g.note(fields[len(fields)-1])
	}
}

func (g *tsortGraph) write(stdout io.Writer) error {
	incoming := map[string]int{}
	for _, name := range g.order {
		for _, target := range g.after[name] {
			incoming[target]++
		}
	}
	emitted := map[string]bool{}
	for range g.order {
		next := ""
		for _, name := range g.order {
			if !emitted[name] && incoming[name] == 0 {
				next = name
				break
			}
		}
		if next == "" {
			// Everything remaining is in a cycle. Reported rather than silently
			// truncated: a partial order looks exactly like a complete one.
			return fmt.Errorf("input contains a loop")
		}
		emitted[next] = true
		for _, target := range g.after[next] {
			incoming[target]--
		}
		if _, err := fmt.Fprintln(stdout, next); err != nil {
			return err
		}
	}
	return nil
}

// newStringsApplet prints runs of printable characters, which is how a binary is
// read without a hex dump. -n sets the shortest run, default 4.
func newStringsApplet() Applet {
	return simpleApplet{name: "strings", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "afo", "nt")
		if err != nil {
			return err
		}
		least := 4
		if options.has('n') {
			parsed, err := strconv.Atoi(options.value('n'))
			if err != nil || parsed <= 0 {
				return fmt.Errorf("invalid number '%s'", options.value('n'))
			}
			least = parsed
		}
		radix, err := stringsRadix(options)
		if err != nil {
			return err
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			return writePrintableRuns(stdout, reader, least, radix)
		})
	}}
}

func stringsRadix(options appletOptions) (byte, error) {
	if options.has('t') {
		value := options.value('t')
		if len(value) != 1 || !strings.Contains("doxX", value) {
			return 0, fmt.Errorf("invalid radix '%s'", value)
		}
		return value[0], nil
	}
	if options.has('o') {
		return 'o', nil
	}
	return 0, nil
}

func writePrintableRuns(stdout io.Writer, reader io.Reader, least int, radix byte) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	run := make([]byte, 0, 64)
	flush := func(end int) error {
		text := string(run)
		run = run[:0]
		if len(text) < least {
			return nil
		}
		if radix == 0 {
			_, err := fmt.Fprintln(stdout, text)
			return err
		}
		formats := map[byte]string{'d': "%7d %s\n", 'o': "%7o %s\n", 'x': "%7x %s\n", 'X': "%7X %s\n"}
		_, err := fmt.Fprintf(stdout, formats[radix], end-len(text), text)
		return err
	}
	for index, b := range data {
		// Printable ASCII plus tab, which is what both references treat as text.
		if b == '\t' || (b >= 0x20 && b < 0x7f) {
			run = append(run, b)
			continue
		}
		if err := flush(index); err != nil {
			return err
		}
	}
	return flush(len(data))
}
