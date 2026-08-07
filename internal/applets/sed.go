package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// sed reads its operands as files and falls back to stdin when there are none,
// which is what every other filter here does. It used to read stdin and nothing
// else, so `sed 's/a/b/' notes.txt` exited 1 with no diagnostic at all -- the
// operand was neither used nor refused.
//
// An unreadable operand is warned about and skipped, leaving status 1 behind,
// which is what busybox does with fopen_or_warn and G.exitcode
// (editors/sed.c:1061-1063).
func newSedApplet() Applet {
	return simpleApplet{name: "sed", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		if len(args) == 0 {
			return missingOperand()
		}
		substitute, err := parseSedSubstitute(args[0])
		if err != nil {
			return err
		}
		if len(args) == 1 {
			return substitute.apply(stdin, stdout)
		}
		return substitute.applyToFiles(ctx, args[1:], stdout, stderr)
	}}
}

type sedSubstitute struct {
	pattern     string
	replacement string
	global      bool
	occurrence  int
}

func (s sedSubstitute) applyToFiles(ctx context.Context, operands []string, stdout, stderr io.Writer) error {
	view := ProcessViewFromContext(ctx)
	failed := false
	for _, operand := range operands {
		file, err := OpenProcessInput(ctx, view, operand)
		if err != nil {
			fmt.Fprintf(stderr, "sed: %s\n", cannotOpen(operand, err))
			failed = true
			continue
		}
		applyErr := s.apply(file, stdout)
		closeErr := file.Close()
		if applyErr != nil {
			return applyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if failed {
		return ExitStatus(1)
	}
	return nil
}

func (s sedSubstitute) apply(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		if _, err := fmt.Fprintln(output, s.replace(scanner.Text())); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// replace does what the s/// flags say: the Nth match when a number is given,
// every match from there on when g is, and the first match otherwise. Only the
// first was ever done, so `s/a/b/g` was rejected as malformed rather than
// applied -- loud, at least, but not usable.
func (s sedSubstitute) replace(line string) string {
	first := max(s.occurrence, 1)
	var out strings.Builder
	rest := line
	matches := 0
	for {
		index := strings.Index(rest, s.pattern)
		if index < 0 {
			break
		}
		matches++
		out.WriteString(rest[:index])
		if matches < first {
			out.WriteString(s.pattern)
		} else {
			out.WriteString(s.replacement)
		}
		rest = rest[index+len(s.pattern):]
		if matches >= first && !s.global {
			break
		}
	}
	out.WriteString(rest)
	return out.String()
}

func parseSedSubstitute(script string) (sedSubstitute, error) {
	if len(script) < 3 || script[0] != 's' {
		return sedSubstitute{}, fmt.Errorf("unsupported sed script: %s", script)
	}
	delimiter := script[1]
	parts := strings.SplitN(script[2:], string(delimiter), 3)
	if len(parts) != 3 || parts[0] == "" {
		return sedSubstitute{}, fmt.Errorf("malformed sed substitute: %s", script)
	}
	global, occurrence, err := parseSedSubstituteFlags(parts[2])
	if err != nil {
		return sedSubstitute{}, err
	}
	return sedSubstitute{pattern: parts[0], replacement: parts[1], global: global, occurrence: occurrence}, nil
}

func parseSedSubstituteFlags(flags string) (bool, int, error) {
	global := false
	occurrence := 0
	for index := 0; index < len(flags); index++ {
		if flags[index] == 'g' {
			global = true
			continue
		}
		if flags[index] < '0' || flags[index] > '9' {
			return false, 0, fmt.Errorf("unknown option to `s': %c", flags[index])
		}
		end := index
		for end < len(flags) && flags[end] >= '0' && flags[end] <= '9' {
			end++
		}
		value, err := strconv.Atoi(flags[index:end])
		if err != nil || value == 0 {
			return false, 0, fmt.Errorf("number option to `s' command may not be zero")
		}
		occurrence = value
		index = end - 1
	}
	return global, occurrence, nil
}
