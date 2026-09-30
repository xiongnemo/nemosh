package applets

import (
	"context"
	"errors"
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
		options, err := sedArgs(ctx, args)
		if err != nil {
			return err
		}
		// Parsed whole before a line is read, so a caller piping sed into
		// something else never receives half an answer.
		program, err := parseSedProgram(options.scripts, options.quiet, options.extended)
		if err != nil {
			return err
		}
		if options.inPlace {
			return runSedInPlace(ctx, program, options.operands, options.suffix, stderr)
		}
		return program.run(ctx, options.operands, stdin, stdout, stderr)
	}}
}

// run applies the program to the operands as one stream.
func (p *sedProgram) run(ctx context.Context, operands []string, stdin io.Reader, stdout, stderr io.Writer) error {
	p.view = ProcessViewFromContext(ctx)
	closeFiles, err := p.openWriteFiles(ctx)
	if err != nil {
		return err
	}
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
				file, err := OpenProcessOperand(ctx, view, name, stdin)
				if err != nil {
					return nil, cannotOpen(name, err)
				}
				// Decoded, because a regular expression cannot match across UTF-16
				// code units: `sed s/hello/x/` over a file Notepad wrote used to match
				// nothing and copy it through. Printed output is UTF-8, the same rule
				// grep follows; `-i` is the case that has to put the encoding back, and
				// it does -- see sed_inplace.go.
				return decodedCloser{Reader: decodeTextInput(file), closer: file}, nil
			})
		}
	}
	runErr := p.execute(stream, stdout)
	closeErr := errors.Join(stream.Close(), closeFiles())
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
	// The writer holds each newline back until it knows something follows, so the
	// last one can be withheld if the input's last line had none. See sed_output.go
	// for why the rule cannot be per line.
	output := newSedOutput(stdout)
	// Settled once, on every way out of the loop -- including `q`, which returns
	// from the middle. An error path leaves it unsettled on purpose: half a line
	// plus a newline is no better than half a line.
	finish := func(err error) error {
		if err != nil {
			return err
		}
		return output.close()
	}
	number := 0
	hold := ""
	for {
		line, ended, ok, err := stream.Next()
		if err != nil {
			return err
		}
		if !ok {
			return finish(nil)
		}
		number++
		cycle := &sedCycle{
			line:   line,
			ended:  ended,
			number: number,
			isLast: stream.AtLast(),
			output: output,
			quiet:  p.quiet,
			hold:   hold,
			stream: stream,
			view:   p.view,
		}
		control, err := runSedProgram(p, cycle)
		if err != nil {
			return err
		}
		// The hold space survives the cycle; the pattern space does not. `n` and
		// `N` may also have advanced the line counter past this cycle's start.
		hold, number = cycle.hold, cycle.number
		// Without -n the pattern space is printed at the end of the script, which
		// is why `p` without -n duplicates a line rather than printing it once.
		// `d` and `q` both skip it: `d` discarded the line, and `q` already wrote
		// it on the way out.
		if control == sedNext && !p.quiet {
			if err := output.writeLine(cycle.line, cycle.ended); err != nil {
				return err
			}
		}
		// What `a` queued goes out after the pattern space, and regardless of how
		// the cycle ended: the text belongs after the line whether or not the line
		// itself was printed.
		if err := cycle.flushAppended(); err != nil {
			return err
		}
		if control == sedQuit {
			return finish(nil)
		}
	}
}

type sedSubstitute struct {
	// pattern is compiled, not searched for. It used to be a literal needle handed to
	// strings.Index; see sed_regex.go for what that cost.
	pattern     *regexp.Regexp
	replacement string
	global      bool
	occurrence  int
	// print is the p flag: the pattern space is written when a replacement was made.
	print bool
	// writeName is the w flag's FILE, where the pattern space goes too; the command's
	// writeTo is the file, opened when the run starts.
	writeName string
}

// replace does what the s/// flags say: the Nth match when a number is given, every match
// from there on when g is, and the first match otherwise.
//
// Written over the match positions rather than with ReplaceAll, because "the second match
// onwards" is not something ReplaceAll can express. An empty match counts as a match, which
// is what makes `s/[0-9]*//` replace the empty string at the start of the line and leave the
// digits alone -- measured against busybox, which does the same.
//
// It answers whether a replacement was made, which is what p and t ask: `s/a/a/` makes one,
// though the line is as it was, as busybox's do_subst_command answers.
func (s sedSubstitute) replace(line string) (string, bool) {
	matches := s.pattern.FindAllStringSubmatchIndex(line, -1)
	if matches == nil {
		return line, false
	}
	first := max(s.occurrence, 1)
	var out strings.Builder
	written, made := 0, false
	for number, match := range matches {
		if number+1 < first {
			continue
		}
		out.WriteString(line[written:match[0]])
		out.Write(s.pattern.ExpandString(nil, s.replacement, line, match))
		written, made = match[1], true
		if !s.global {
			break
		}
	}
	out.WriteString(line[written:])
	return out.String(), made
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
	substitute := sedSubstitute{replacement: translateReplacement(replacement)}
	// w FILE ends the flags, and the command: FILE runs to the end of the line.
	if strings.HasPrefix(rest, "w") {
		if substitute.writeName, rest, err = parseSedFileName(rest[1:]); err != nil {
			return sedSubstitute{}, "", err
		}
	}
	ignoreCase, err := parseSedSubstituteFlags(flags, &substitute)
	if err != nil {
		return sedSubstitute{}, "", err
	}
	if substitute.pattern, err = compileSedPattern(pattern, extended, ignoreCase); err != nil {
		return sedSubstitute{}, "", err
	}
	return substitute, rest, nil
}

// splitSedSubstituteFlags takes the flag letters that follow the closing
// delimiter, stopping at whatever ends the command.
//
// This is why the substitution parser had to be rewritten to report a remainder:
// with `;` separating commands, the tail after `s/a/b/` may be another command
// rather than the end of the script, and the old splitter consumed everything.
//
// p is one of them. It was not, so the splitter stopped at it and the p was read as the next
// command, a print of every line: `sed -n 's/a/X/p'` printed the lines it changed nothing on.
func splitSedSubstituteFlags(rest string) (string, string) {
	end := 0
	for end < len(rest) && strings.IndexByte("gpiI0123456789", rest[end]) >= 0 {
		end++
	}
	return rest[:end], rest[end:]
}

// parseSedSubstituteFlags reads the letters after the closing delimiter.
//
// `i` and `I` are case-insensitive matching, which busybox has and this refused
// -- and refused incoherently: splitSedSubstituteFlags *consumed* the letter and
// then this rejected it, so the two halves of one parser disagreed about which
// flags exist. Either the splitter should have stopped at `i` or this should have
// accepted it; busybox accepts it, so this does. The rest go into the substitution;
// ignoreCase is answered, since the pattern is compiled with it.
func parseSedSubstituteFlags(flags string, into *sedSubstitute) (bool, error) {
	ignoreCase := false
	for index := 0; index < len(flags); index++ {
		switch {
		case flags[index] == 'g':
			into.global = true
			continue
		case flags[index] == 'p':
			into.print = true
			continue
		case flags[index] == 'i' || flags[index] == 'I':
			ignoreCase = true
			continue
		case flags[index] < '0' || flags[index] > '9':
			return false, fmt.Errorf("unknown option to `s': %c", flags[index])
		}
		end := index
		for end < len(flags) && flags[end] >= '0' && flags[end] <= '9' {
			end++
		}
		value, err := strconv.Atoi(flags[index:end])
		if err != nil || value == 0 {
			return false, fmt.Errorf("number option to `s' command may not be zero")
		}
		into.occurrence = value
		index = end - 1
	}
	return ignoreCase, nil
}
