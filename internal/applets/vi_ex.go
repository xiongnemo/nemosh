package applets

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/xiongnemo/nemosh/internal/version"
)

// The line typed after :, busybox's colon(). Enter runs it; Escape abandons it, as in vim --
// busybox's get_input_line runs it on Escape too, so `:q!<Esc>` quit -- and so does a
// backspace past the colon.

func (e *viEditor) commandLineKey(key rune) {
	switch {
	case key == viEnter || key == '\n':
		prompt, line, before := e.command[0], string(e.command[1:]), e.cursor()
		e.mode, e.command = viNormal, nil
		switch {
		case prompt == ':':
			e.exCommand(line)
		case e.op != 0:
			e.searchInput = line
			e.operate(prompt, 0)
		case e.searchFor(prompt, line) && e.cursor() != before:
			e.context = viMark{pos: before, set: true}
		}
		if e.op == 0 {
			e.afterCommand()
		}
	case key == viEscape || key == 3:
		e.mode, e.command, e.op = viNormal, nil, 0
	case isViBackspace(key):
		e.command = e.command[:len(e.command)-1]
		if len(e.command) == 0 {
			e.mode, e.op = viNormal, 0
		}
	case key >= ' ' || key == '\t':
		e.command = append(e.command, key)
	}
}

// viExCommands are the names an ex command is known by, and how much of each must be typed; a
// shorter name is matched as a prefix, as busybox matches it. p is vim's print, where busybox's
// was the previous file, :prev here.
var viExCommands = [...]struct {
	name  string
	least int
}{
	{"delete", 1}, {"edit", 1}, {"file", 1}, {"list", 1}, {"next", 1}, {"Next", 1}, {"print", 1},
	{"previous", 4}, {"quit", 1}, {"read", 1}, {"rewind", 3}, {"substitute", 1}, {"set", 2},
	{"version", 2}, {"write", 1}, {"wq", 2}, {"wnext", 2}, {"xit", 1}, {"yank", 1},
}

// exCommand runs one ex command: its addresses, a name, a ! after it, and an argument.
func (e *viEditor) exCommand(line string) {
	line = strings.TrimLeft(line, ": \t")
	if line == "" || line[0] == '"' {
		return
	}
	first, last, given, rest, ok := e.exAddresses(line)
	if !ok {
		return
	}
	rest = strings.TrimLeft(rest, " \t")
	end := strings.IndexFunc(rest, func(r rune) bool { return !unicode.IsLetter(r) })
	switch {
	case end < 0:
		end = len(rest)
	case end == 0 && rest != "":
		end = 1
	}
	name, argument := rest[:end], rest[end:]
	force := strings.HasPrefix(argument, "!") && end > 0 && unicode.IsLetter(rune(name[0]))
	if force {
		argument = argument[1:]
	}
	argument = strings.TrimLeft(argument, " \t")
	if name == "" {
		// :N goes to line N, the last when there are fewer, vim's; busybox's refused.
		if given > 0 {
			e.context = viMark{pos: e.cursor(), set: true}
			e.row = max(1, min(last, len(e.lines))) - 1
			e.col = e.firstNonBlank(e.row)
		}
		return
	}
	if given > 0 && (last < 0 || last > len(e.lines) || first < 0 || first > last) {
		e.say(true, "Invalid range")
		return
	}
	// Line 0 is before the first, which only :0r reads into; to the rest it is the first.
	after := last
	first, last = max(first, 1), max(last, 1)
	full := name
	for _, command := range viExCommands {
		if len(name) >= command.least && strings.HasPrefix(command.name, name) {
			full = command.name
			break
		}
	}
	switch full {
	case "=":
		// :=, vim's, is the last line; :.= the one given.
		if given == 0 {
			last = len(e.lines)
		}
		e.say(false, "%d", last)
	case "delete", "yank":
		e.exLines(full, first, last, argument)
	case "print", "list":
		e.printLines(full == "list", first, last)
	case "substitute":
		e.substitute(first, last, argument)
	case "set":
		e.setOptions(argument)
	case "!":
		e.shellCommand(given, argument)
	case "edit":
		e.exEdit(name, force, argument)
	case "file":
		e.exFile(given, argument)
	case "read":
		if given == 0 {
			after = e.row + 1
		}
		e.exRead(after, argument)
	case "next", "Next", "previous", "rewind":
		e.exSwitch(full, name, force)
	case "version":
		e.say(false, "nemosh vi, busybox-w32's with vim's ways, %s", version.Describe())
	case "quit":
		e.exQuit(name, force)
	case "wq", "xit", "wnext":
		// :x writes only a buffer that changed, :wq and :wn always; then each goes on if that
		// went well, :wn to the next file.
		if (full != "xit" || e.isModified()) && !e.exWrite(argument, force) {
			return
		}
		if full == "wnext" {
			e.exSwitch(full, name, false)
			return
		}
		e.exQuit(name, force)
	case "write":
		e.exWrite(argument, force)
	default:
		e.say(true, "'%s' is not implemented", name)
	}
}

// exLines is :d and :y over lines first to last, or as many lines as a count after them says,
// from the last, vim's.
func (e *viEditor) exLines(command string, first, last int, argument string) {
	if count, err := strconv.Atoi(argument); err == nil && count > 0 {
		first, last = last, min(last+count-1, len(e.lines))
	}
	text := e.linesText(first-1, last-1)
	if command == "yank" {
		e.store("", text, true)
		e.say(false, "Yank %d lines (%d chars) into [%c]", last-first+1, len(text), registerName(e.register))
		return
	}
	e.store("", text, true)
	e.replaceLines(first-1, last, nil)
	e.row = min(first-1, len(e.lines)-1)
	e.col = e.firstNonBlank(e.row)
}

// printLines is :p and :l, the cursor going to the last: one line on the status line, more
// above it until Enter. :l shows a tab as ^I, any control character as ^X, and the end as $.
func (e *viEditor) printLines(list bool, first, last int) {
	var shown []string
	for _, line := range e.lines[first-1 : last] {
		text := string(line)
		if list {
			var out strings.Builder
			for _, r := range line {
				if r < ' ' || r == 0x7f {
					fmt.Fprintf(&out, "^%c", r^0x40)
					continue
				}
				out.WriteRune(r)
			}
			text = out.String() + "$"
		}
		shown = append(shown, text)
	}
	e.row = last - 1
	e.col = e.firstNonBlank(e.row)
	e.showLines(shown)
}
