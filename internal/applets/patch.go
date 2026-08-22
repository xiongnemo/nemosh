package applets

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// patch: applies what diff writes.
//
// The pair is kept together deliberately. Shipping patch against a diff whose
// output shape later changed would break it silently, so the tests round-trip the
// two against each other rather than testing each alone.
//
// A hunk that does not apply is **refused with its line number**, not applied
// approximately. patch's traditional fuzz -- shifting a hunk until it fits -- is
// how a patch lands in the wrong place and the wrong place still compiles.

func newPatchApplet() Applet {
	return simpleApplet{name: "patch", runContext: func(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		options, operands, err := parseAppletOptions(args, "RNEfl", "pi")
		if err != nil {
			return err
		}
		strip := 0
		if options.has('p') {
			parsed, err := strconv.Atoi(options.value('p'))
			if err != nil || parsed < 0 {
				return fmt.Errorf("invalid strip count '%s'", options.value('p'))
			}
			strip = parsed
		}
		source := stdin
		if options.has('i') {
			file, err := OpenProcessInput(ctx, ProcessViewFromContext(ctx), options.value('i'))
			if err != nil {
				return operandFailure(options.value('i'), err)
			}
			defer file.Close()
			source = file
		}
		hunkSets, err := parseUnifiedPatch(source)
		if err != nil {
			return err
		}
		return applyPatchSets(ctx, hunkSets, patchOptions{
			strip:   strip,
			reverse: options.has('R'),
			target:  firstOperand(operands),
		}, stdout, stderr)
	}}
}

func firstOperand(operands []string) string {
	if len(operands) == 0 {
		return ""
	}
	return operands[0]
}

type patchOptions struct {
	strip   int
	reverse bool
	target  string
}

// patchHunk is one `@@` block.
type patchHunk struct {
	leftStart  int
	rightStart int
	lines      []string
}

// patchSet is every hunk for one file.
type patchSet struct {
	oldName string
	newName string
	hunks   []patchHunk
}

// parseUnifiedPatch reads a unified diff into per-file hunk sets.
func parseUnifiedPatch(reader io.Reader) ([]patchSet, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTextLine)
	var sets []patchSet
	var current *patchSet
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		switch {
		case strings.HasPrefix(line, "--- "):
			sets = append(sets, patchSet{oldName: patchName(line[4:])})
			current = &sets[len(sets)-1]
		case strings.HasPrefix(line, "+++ ") && current != nil:
			current.newName = patchName(line[4:])
		case strings.HasPrefix(line, "@@") && current != nil:
			hunk, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			current.hunks = append(current.hunks, hunk)
		case current != nil && len(current.hunks) > 0 && isPatchBodyLine(line):
			last := &current.hunks[len(current.hunks)-1]
			last.lines = append(last.lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(sets) == 0 {
		return nil, fmt.Errorf("only garbage was found in the patch input")
	}
	return sets, nil
}

// isPatchBodyLine reports whether a line belongs to the hunk being read.
//
// An empty line counts as a context line holding nothing: diff writes a bare
// space for it, but many mailers strip trailing whitespace, so a body line that
// is entirely empty has to be accepted as " ".
func isPatchBodyLine(line string) bool {
	if line == "" {
		return true
	}
	switch line[0] {
	case ' ', '+', '-':
		return true
	}
	return false
}

// patchName takes the filename out of a header line, dropping any timestamp diff
// may have written after a tab.
func patchName(rest string) string {
	name, _, _ := strings.Cut(rest, "\t")
	return strings.TrimSpace(name)
}

func parseHunkHeader(line string) (patchHunk, error) {
	// `@@ -3,4 +3,5 @@ optional trailing text`
	fields := strings.Fields(line)
	if len(fields) < 3 || !strings.HasPrefix(fields[1], "-") || !strings.HasPrefix(fields[2], "+") {
		return patchHunk{}, fmt.Errorf("malformed hunk header: %s", line)
	}
	left, err := hunkStart(fields[1][1:])
	if err != nil {
		return patchHunk{}, fmt.Errorf("malformed hunk header: %s", line)
	}
	right, err := hunkStart(fields[2][1:])
	if err != nil {
		return patchHunk{}, fmt.Errorf("malformed hunk header: %s", line)
	}
	return patchHunk{leftStart: left, rightStart: right}, nil
}

func hunkStart(field string) (int, error) {
	start, _, _ := strings.Cut(field, ",")
	return strconv.Atoi(start)
}

// stripPathComponents applies -p, which is how a patch made in one tree is
// applied in another.
func stripPathComponents(name string, count int) string {
	name = strings.ReplaceAll(name, `\`, "/")
	for range count {
		_, rest, found := strings.Cut(name, "/")
		if !found {
			return name
		}
		name = rest
	}
	return name
}

func applyPatchSets(ctx context.Context, sets []patchSet, options patchOptions, stdout, stderr io.Writer) error {
	view := ProcessViewFromContext(ctx)
	for _, set := range sets {
		target := options.target
		if target == "" {
			// The name comes from the patch, so it is checked the way an archive
			// entry is: `--- ../../etc/passwd` is the same attack.
			candidate := stripPathComponents(set.oldName, options.strip)
			safe, err := safeArchivePath(candidate)
			if err != nil {
				return err
			}
			target = safe
		}
		native, err := resolveHostPath(view, target)
		if err != nil {
			return operandFailure(target, err)
		}
		original, err := os.ReadFile(native)
		if err != nil {
			return operandFailure(target, err)
		}
		patched, err := applyHunks(splitPatchLines(string(original)), set.hunks, options.reverse)
		if err != nil {
			return fmt.Errorf("%s: %v", target, err)
		}
		if _, err := fmt.Fprintf(stderr, "patching file %s\n", target); err != nil {
			return err
		}
		if err := os.WriteFile(native, []byte(strings.Join(patched, "\n")+trailingNewline(string(original))), 0o644); err != nil {
			return operandFailure(target, err)
		}
	}
	return nil
}

func splitPatchLines(text string) []string {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func trailingNewline(text string) string {
	if strings.HasSuffix(text, "\n") {
		return "\n"
	}
	return ""
}
