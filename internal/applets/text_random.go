package applets

import (
	"context"
	"encoding/base32"
	"fmt"
	"io"
	"math/rand/v2"
	"strconv"
	"strings"
)

// base32 and shuf. Split from text_columns.go for the size ceiling; base32 is
// base64's sibling and shuf is the only applet here whose output is deliberately
// not reproducible.

// newBase32Applet is base64's sibling, wrapping at 76 columns the same way.
func newBase32Applet() Applet {
	return simpleApplet{name: "base32", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, paths, err := parseAppletOptions(args, "di", "w")
		if err != nil {
			return err
		}
		width := base64Wrap
		if options.has('w') {
			parsed, err := strconv.Atoi(options.value('w'))
			if err != nil || parsed < 0 {
				return fmt.Errorf("invalid wrap size: %s", options.value('w'))
			}
			width = parsed
		}
		return eachTextInput(ctx, paths, stdin, func(reader io.Reader) error {
			if options.has('d') {
				return decodeBase32(reader, stdout, options.has('i'))
			}
			return encodeBase32(reader, stdout, width)
		})
	}}
}

func encodeBase32(reader io.Reader, stdout io.Writer, width int) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	return writeWrapped(stdout, base32.StdEncoding.EncodeToString(data), width)
}

func decodeBase32(reader io.Reader, stdout io.Writer, ignoreGarbage bool) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	var cleaned strings.Builder
	for _, character := range string(data) {
		switch {
		case character == '\n' || character == '\r':
			continue
		case ignoreGarbage && !isBase32Rune(character):
			continue
		}
		cleaned.WriteRune(character)
	}
	// Padding is optional in the wild, so a run that is short of a block is
	// padded rather than refused -- the same leniency base64 -i offers.
	text := cleaned.String()
	if remainder := len(text) % 8; remainder != 0 {
		text += strings.Repeat("=", 8-remainder)
	}
	decoded, err := base32.StdEncoding.DecodeString(text)
	if err != nil {
		return fmt.Errorf("invalid input")
	}
	_, err = stdout.Write(decoded)
	return err
}

func isBase32Rune(character rune) bool {
	return (character >= 'A' && character <= 'Z') || (character >= '2' && character <= '7') || character == '='
}

// newShufApplet permutes lines. -n takes at most that many, -e treats the
// operands as the input, -i generates a range, -z uses NUL terminators.
//
// The order is genuinely random, so the tests assert the *set* and the count
// rather than a sequence -- a test that pinned an order would either be wrong or
// would prove the shuffle does not shuffle.
func newShufApplet() Applet {
	return simpleApplet{name: "shuf", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "ez", "ni")
		if err != nil {
			return err
		}
		lines, err := shufInput(ctx, options, operands, stdin)
		if err != nil {
			return err
		}
		rand.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
		if options.has('n') {
			count, err := strconv.Atoi(options.value('n'))
			if err != nil || count < 0 {
				return fmt.Errorf("invalid number '%s'", options.value('n'))
			}
			lines = lines[:min(count, len(lines))]
		}
		terminator := "\n"
		if options.has('z') {
			terminator = "\x00"
		}
		for _, line := range lines {
			if _, err := io.WriteString(stdout, line+terminator); err != nil {
				return err
			}
		}
		return nil
	}}
}

func shufInput(ctx context.Context, options appletOptions, operands []string, stdin io.Reader) ([]string, error) {
	if options.has('e') {
		return operands, nil
	}
	if options.has('i') {
		low, high, found := strings.Cut(options.value('i'), "-")
		first, firstErr := strconv.Atoi(low)
		last, lastErr := strconv.Atoi(high)
		if !found || firstErr != nil || lastErr != nil || last < first {
			return nil, fmt.Errorf("invalid range '%s'", options.value('i'))
		}
		lines := make([]string, 0, last-first+1)
		for value := first; value <= last; value++ {
			lines = append(lines, strconv.Itoa(value))
		}
		return lines, nil
	}
	var lines []string
	err := eachTextInput(ctx, operands, stdin, func(reader io.Reader) error {
		return eachLine(reader, func(line, _ string) error {
			lines = append(lines, line)
			return nil
		})
	})
	return lines, err
}
