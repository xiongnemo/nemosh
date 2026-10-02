package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// factor, tsort and strings: three small tools in one file because each is a
// handful of lines and they share nothing but the operand seam. fold is in fold.go.
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
		_, operands, err := parseAppletOptions(ctx, args, "", "")
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

// newTsortApplet topologically sorts `before after` pairs.
//
// The words are one stream, paired across lines as both references pair them, and an odd one
// out is `odd input` before anything is written. Each line was paired on its own, and a word
// left over named an item alone, where both refuse it: `solo solo` is how an item with no
// order is written. One FILE is read, or `-`; a second is an extra operand, where busybox's
// shows its usage.
func newTsortApplet() Applet {
	return simpleApplet{name: "tsort", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		_, paths, err := parseAppletOptions(ctx, args, "", "")
		if err != nil {
			return err
		}
		if len(paths) > 1 {
			return fmt.Errorf("extra operand '%s'", paths[1])
		}
		var words []string
		collect := func(reader io.Reader) error {
			return eachLine(reader, func(line, _ string) error {
				words = append(words, strings.Fields(line)...)
				return nil
			})
		}
		if err := eachTextInputQuoted(ctx, paths, stdin, collect); err != nil {
			return err
		}
		if len(words)%2 == 1 {
			return errors.New("odd input")
		}
		graph := &tsortGraph{seen: map[string]bool{}, after: map[string][]string{}}
		graph.addPairs(words)
		return graph.write(stdout, stderr)
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
}

// write writes the items, each after those that come before it. Where only a cycle is left it
// is said, `cycle at NAME`, and broken at its first item, and the rest is written, status 1,
// as both references write it. The output ended at the cycle, which looked exactly like a
// complete order but for the status.
func (g *tsortGraph) write(stdout, stderr io.Writer) error {
	incoming := map[string]int{}
	for _, name := range g.order {
		for _, target := range g.after[name] {
			incoming[target]++
		}
	}
	emitted, cycles := map[string]bool{}, false
	for range g.order {
		next, first := "", ""
		for _, name := range g.order {
			if emitted[name] {
				continue
			}
			if first == "" {
				first = name
			}
			if incoming[name] == 0 {
				next = name
				break
			}
		}
		if next == "" {
			next, cycles = first, true
			fmt.Fprintf(stderr, "tsort: cycle at %s\n", next)
		}
		emitted[next] = true
		for _, target := range g.after[next] {
			incoming[target]--
		}
		if _, err := fmt.Fprintln(stdout, next); err != nil {
			return err
		}
	}
	if cycles {
		return ErrExitFalse
	}
	return nil
}

// newStringsApplet prints runs of printable characters, which is how a binary is
// read without a hex dump. -n sets the shortest run, default 4.
func newStringsApplet() Applet {
	return simpleApplet{name: "strings", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(ctx, args, "afo", "nt")
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
		return eachTextFile(ctx, paths, stdin, func(reader io.Reader) error {
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
