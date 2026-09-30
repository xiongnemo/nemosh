package applets

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The file commands and `l`, busybox's (editors/sed.c) and POSIX's.
//
// `r FILE` queues FILE's lines to be written at the end of the cycle, as `a` queues its text,
// and a FILE that cannot be read queues nothing. `w FILE` and s///'s w flag write the pattern
// space to FILE, which is made empty when the run starts, whether or not a line reaches it,
// and is one file for every command that names it. A FILE runs to the end of the line, as
// busybox reads it, so `w out; p` writes to a file called `out; p`. `l` writes the pattern
// space unambiguously: a backslash doubled, \a \b \f \n \r \t \v, a byte that is no printable
// character in octal, a line longer than 69 characters split with a backslash, and a `$` at
// the end. They were `unsupported command r`, `w` and `l`.

// parseSedFileName reads a file command's FILE: blanks skipped, then everything to the end of
// the line.
func parseSedFileName(script string) (string, string, error) {
	name := strings.TrimLeft(script, " \t")
	rest := ""
	if end := strings.IndexByte(name, '\n'); end >= 0 {
		name, rest = name[:end], name[end:]
	}
	if name == "" {
		return "", "", errors.New("empty filename")
	}
	return name, rest, nil
}

// sedWriteFile is one `w` FILE: the output it is written through, which ends its lines as the
// standard output's does, and the file under it, which may be a device: `w /dev/stderr`.
type sedWriteFile struct {
	output *sedOutput
	file   io.WriteCloser
}

// openWriteFiles makes every FILE a `w` or an s///w names empty, once for each name, and gives
// each command its writer. The closer closes them all.
func (p *sedProgram) openWriteFiles(ctx context.Context) (func() error, error) {
	view := ProcessViewFromContext(ctx)
	opened := map[string]*sedWriteFile{}
	closeAll := func() error {
		var err error
		for _, target := range opened {
			err = errors.Join(err, target.file.Close())
		}
		return err
	}
	for _, command := range p.instructions {
		name := command.file
		if command.action == 's' {
			name = command.substitute.writeName
		}
		if name == "" || command.action == 'r' {
			continue
		}
		target, ok := opened[name]
		if !ok {
			file, err := openProcessOutput(view, name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, createMode(view, 0o666))
			if err != nil {
				return nil, errors.Join(cannotOpen(name, err), closeAll())
			}
			target = &sedWriteFile{output: newSedOutput(file), file: file}
			opened[name] = target
		}
		command.writeTo = target
	}
	return closeAll, nil
}

// runSedFileCommand is r, w and l.
func runSedFileCommand(command *sedCommand, cycle *sedCycle) error {
	switch command.action {
	case 'r':
		file, err := openProcessInput(cycle.view, command.file)
		if err != nil {
			return nil
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			cycle.appended = append(cycle.appended, strings.TrimSuffix(scanner.Text(), "\r"))
		}
		return nil
	case 'w':
		return cycle.writeFile(command.writeTo)
	}
	for _, line := range sedListLines(cycle.line) {
		if err := cycle.write(line); err != nil {
			return err
		}
	}
	return nil
}

// writeFile writes the pattern space to a `w` FILE.
func (c *sedCycle) writeFile(target *sedWriteFile) error {
	if target == nil {
		return nil
	}
	return target.output.writeLine(c.line, c.ended)
}

// sedListLines is `l`'s output for a pattern space: escaped, split at 69 characters with a
// backslash, and ended with `$`. An escape is never split.
func sedListLines(text string) []string {
	var lines []string
	var line strings.Builder
	for len(text) > 0 {
		character, size := utf8.DecodeRuneInString(text)
		piece := sedListEscape(text[:size], character)
		text = text[size:]
		if line.Len()+len(piece) > 69 {
			lines = append(lines, line.String()+`\`)
			line.Reset()
		}
		line.WriteString(piece)
	}
	return append(lines, line.String()+"$")
}

// sedListEscape is how `l` writes one character, or one byte that is no character.
func sedListEscape(raw string, character rune) string {
	switch character {
	case '\\':
		return `\\`
	case '\a':
		return `\a`
	case '\b':
		return `\b`
	case '\f':
		return `\f`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\t':
		return `\t`
	case '\v':
		return `\v`
	}
	if character != utf8.RuneError && unicode.IsPrint(character) {
		return raw
	}
	var escaped strings.Builder
	for index := 0; index < len(raw); index++ {
		fmt.Fprintf(&escaped, `\%03o`, raw[index])
	}
	return escaped.String()
}
