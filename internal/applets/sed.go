package applets

import (
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
		scripts, operands, quiet, extended, err := sedArgs(args)
		if err != nil {
			return err
		}
		// Parsed whole before a line is read, so a caller piping sed into
		// something else never receives half an answer.
		program, err := parseSedProgram(scripts, quiet, extended)
		if err != nil {
			return err
		}
		return program.run(ctx, operands, stdin, stdout, stderr)
	}}
}

// run applies the program to the operands as one stream.
func (p *sedProgram) run(ctx context.Context, operands []string, stdin io.Reader, stdout, stderr io.Writer) error {
	failed := false
	stream := &sedStream{onOpenError: func(err error) {
		fmt.Fprintf(stderr, "sed: %v\n", err)
		failed = true
	}}
	if len(operands) == 0 {
		stream.openers = []func() (io.ReadCloser, error){
			func() (io.ReadCloser, error) { return io.NopCloser(stdin), nil },
		}
	} else {
		view := ProcessViewFromContext(ctx)
		for _, operand := range operands {
			name := operand
			stream.openers = append(stream.openers, func() (io.ReadCloser, error) {
				file, err := OpenProcessInput(ctx, view, name)
				if err != nil {
					return nil, cannotOpen(name, err)
				}
				return file, nil
			})
		}
	}
	runErr := p.execute(stream, stdout)
	closeErr := stream.Close()
	if runErr != nil {
		return runErr
	}
	if closeErr != nil {
		return closeErr
	}
	// An unreadable operand is warned about and skipped, leaving status 1 behind,
	// which is what busybox does with fopen_or_warn and G.exitcode
	// (editors/sed.c:1061-1063).
	if failed {
		return ExitStatus(1)
	}
	return nil
}

// execute runs the script over every line.
//
// The pattern space is one line: there is no N, D or hold space here, so each
// line is read, transformed, and either printed or not.
func (p *sedProgram) execute(stream *sedStream, stdout io.Writer) error {
	number := 0
	for {
		line, _, ok, err := stream.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		number++
		isLast := stream.AtLast()
		printed, quit, err := p.applyLine(&line, number, isLast, stdout)
		if err != nil {
			return err
		}
		// Without -n the pattern space is printed at the end of the script, which
		// is why `p` without -n duplicates a line rather than printing it once.
		if !printed && !p.quiet {
			if _, err := fmt.Fprintln(stdout, line); err != nil {
				return err
			}
		}
		if quit {
			return nil
		}
	}
}

// applyLine runs every command against one line, reporting whether the line was
// deleted (so the automatic print must not happen) and whether `q` ended the run.
func (p *sedProgram) applyLine(line *string, number int, isLast bool, stdout io.Writer) (bool, bool, error) {
	for _, command := range p.commands {
		if !command.address.selects(*line, number, isLast) {
			continue
		}
		switch command.action {
		case 's':
			*line = command.substitute.replace(*line)
		case 'p':
			if _, err := fmt.Fprintln(stdout, *line); err != nil {
				return true, false, err
			}
		case 'd':
			// The pattern space is discarded and the rest of the script is
			// skipped, which is what makes `sed '2d;s/a/b/'` leave line two
			// untouched rather than substituting into a line it dropped.
			return true, false, nil
		case 'q':
			// The line is still printed unless -n, then the run ends.
			if !p.quiet {
				if _, err := fmt.Fprintln(stdout, *line); err != nil {
					return true, true, err
				}
			}
			return true, true, nil
		}
	}
	return false, false, nil
}

type sedSubstitute struct {
	// pattern is compiled, not searched for. It used to be a literal needle handed to
	// strings.Index; see sed_regex.go for what that cost.
	pattern     *regexp.Regexp
	replacement string
	global      bool
	occurrence  int
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

// parseSedSubstituteCommand reads one `s///` starting at the `s`, and returns
// what is left of the script so a `;` can bring another command after it.
func parseSedSubstituteCommand(script string, extended bool) (sedSubstitute, string, error) {
	if len(script) < 3 || script[0] != 's' {
		return sedSubstitute{}, "", fmt.Errorf("unsupported sed script: %s", script)
	}
	delimiter := script[1]
	pattern, rest, err := readSedDelimited(script[2:], delimiter)
	if err != nil {
		return sedSubstitute{}, "", fmt.Errorf("unterminated `s' command")
	}
	replacement, rest, err := readSedDelimited(rest, delimiter)
	if err != nil {
		return sedSubstitute{}, "", fmt.Errorf("unterminated `s' command")
	}
	if pattern == "" {
		return sedSubstitute{}, "", fmt.Errorf("malformed sed substitute: s%c%s", delimiter, script[2:])
	}
	flags, rest := splitSedSubstituteFlags(rest)
	global, occurrence, err := parseSedSubstituteFlags(flags)
	if err != nil {
		return sedSubstitute{}, "", err
	}
	expression, err := compileSedPattern(pattern, extended)
	if err != nil {
		return sedSubstitute{}, "", err
	}
	return sedSubstitute{
		pattern:     expression,
		replacement: translateReplacement(replacement),
		global:      global,
		occurrence:  occurrence,
	}, rest, nil
}

// splitSedSubstituteFlags takes the flag letters that follow the closing
// delimiter, stopping at whatever ends the command.
//
// This is why the substitution parser had to be rewritten to report a remainder:
// with `;` separating commands, the tail after `s/a/b/` may be another command
// rather than the end of the script, and the old splitter consumed everything.
func splitSedSubstituteFlags(rest string) (string, string) {
	end := 0
	for end < len(rest) && (rest[end] == 'g' || rest[end] == 'i' || rest[end] == 'I' || (rest[end] >= '0' && rest[end] <= '9')) {
		end++
	}
	return rest[:end], rest[end:]
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
