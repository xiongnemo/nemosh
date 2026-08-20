package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
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
	// pattern is compiled, not searched for. It used to be a literal needle handed to
	// strings.Index; see sed_regex.go for what that cost.
	pattern     *regexp.Regexp
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

// replace does what the s/// flags say: the Nth match when a number is given, every match
// from there on when g is, and the first match otherwise.
//
// Written over the match positions rather than with ReplaceAll, because "the second match
// onwards" is not something ReplaceAll can express. An empty match counts as a match, which
// is what makes `s/[0-9]*//` replace the empty string at the start of the line and leave the
// digits alone -- measured against busybox, which does the same.
func (s sedSubstitute) replace(line string) string {
	matches := s.pattern.FindAllStringSubmatchIndex(line, -1)
	if matches == nil {
		return line
	}
	first := max(s.occurrence, 1)
	var out strings.Builder
	written := 0
	for number, match := range matches {
		if number+1 < first {
			continue
		}
		out.WriteString(line[written:match[0]])
		out.Write(s.pattern.ExpandString(nil, s.replacement, line, match))
		written = match[1]
		if !s.global {
			break
		}
	}
	out.WriteString(line[written:])
	return out.String()
}

func parseSedSubstitute(script string) (sedSubstitute, error) {
	if len(script) < 3 || script[0] != 's' {
		return sedSubstitute{}, fmt.Errorf("unsupported sed script: %s", script)
	}
	delimiter := script[1]
	parts, err := splitSedScript(script[2:], delimiter)
	if err != nil {
		return sedSubstitute{}, err
	}
	global, occurrence, err := parseSedSubstituteFlags(parts[2])
	if err != nil {
		return sedSubstitute{}, err
	}
	translated, err := translateBasicRegex(parts[0])
	if err != nil {
		return sedSubstitute{}, err
	}
	expression, err := regexp.Compile(translated)
	if err != nil {
		return sedSubstitute{}, fmt.Errorf("bad pattern '%s': %v", parts[0], err)
	}
	return sedSubstitute{
		pattern:     expression,
		replacement: translateReplacement(parts[1]),
		global:      global,
		occurrence:  occurrence,
	}, nil
}

// splitSedScript cuts `pattern DELIM replacement DELIM flags` at its unescaped delimiters.
//
// strings.SplitN cannot: `s/a\/b/x/` escapes the delimiter, and splitting on every one of
// them left the tail of the pattern to be read as flags -- `sed: unknown option to 's': /`
// for a pattern that was perfectly well formed. The escape is removed as it is passed over,
// because POSIX says an escaped delimiter stands for itself.
func splitSedScript(script string, delimiter byte) ([3]string, error) {
	var parts [3]string
	field := 0
	var current strings.Builder
	for index := 0; index < len(script); index++ {
		switch {
		case script[index] == '\\' && index+1 < len(script) && script[index+1] == delimiter:
			current.WriteByte(delimiter)
			index++
		case script[index] == delimiter:
			if field == 2 {
				return parts, fmt.Errorf("malformed sed substitute: too many %c", delimiter)
			}
			parts[field] = current.String()
			current.Reset()
			field++
		default:
			current.WriteByte(script[index])
		}
	}
	parts[field] = current.String()
	if field < 2 || parts[0] == "" {
		return parts, fmt.Errorf("malformed sed substitute: s%c%s", delimiter, script)
	}
	return parts, nil
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
