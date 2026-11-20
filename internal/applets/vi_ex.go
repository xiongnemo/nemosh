package applets

import (
	"errors"
	"io/fs"
	"os"
	"strings"
	"unicode"
)

// The line typed after :, busybox's colon(). Enter runs it; Escape abandons it, as in vim --
// busybox's get_input_line runs it on Escape too, so `:q!<Esc>` quit -- and so does a
// backspace past the colon.

func (e *viEditor) commandLineKey(key rune) {
	switch {
	case key == viEnter || key == '\n':
		line := string(e.command[1:])
		e.mode, e.command = viNormal, nil
		e.exCommand(line)
		e.clampColumn()
	case key == viEscape || key == 3:
		e.mode, e.command = viNormal, nil
	case isViBackspace(key):
		e.command = e.command[:len(e.command)-1]
		if len(e.command) == 0 {
			e.mode = viNormal
		}
	case key >= ' ' || key == '\t':
		e.command = append(e.command, key)
	}
}

// exCommand runs one ex command: a name, a ! after it, and an argument. A name is matched as
// a prefix, as busybox matches it, so :wr is :write.
func (e *viEditor) exCommand(line string) {
	line = strings.TrimLeft(line, ": \t")
	end := strings.IndexFunc(line, func(r rune) bool { return !unicode.IsLetter(r) })
	if end < 0 {
		end = len(line)
	}
	name, rest := line[:end], line[end:]
	force := strings.HasPrefix(rest, "!")
	if force {
		rest = rest[1:]
	}
	argument := strings.TrimSpace(rest)
	switch {
	case line == "":
		return
	case name == "":
		e.say(true, "'%s' is not implemented", line)
	case strings.HasPrefix("quit", name):
		e.exQuit(name, force)
	case name == "wq" || name == "x" || strings.HasPrefix("xit", name) && len(name) > 1:
		// :x writes only a buffer that changed, :wq always; then each quits if that went well.
		if (name == "wq" || e.isModified()) && !e.exWrite(argument, force) {
			return
		}
		e.quit = true
	case strings.HasPrefix("write", name):
		e.exWrite(argument, force)
	default:
		e.say(true, "'%s' is not implemented", name)
	}
}

func (e *viEditor) exQuit(name string, force bool) {
	if e.isModified() && !force {
		e.say(true, "No write since last change (:%s! overrides)", name)
		return
	}
	e.quit = true
}

// exWrite is :w [FILE], and answers whether it wrote. Writing to the buffer's own file is
// refused when it is read-only, and to another that is there without !, in busybox's words;
// a buffer with no name takes the one it is written to.
func (e *viEditor) exWrite(name string, force bool) bool {
	own := name == "" || name == e.file.name
	if name == "" {
		name = e.file.name
	}
	if name == "" {
		e.say(true, "No current filename")
		return false
	}
	native := e.file.native
	if !own || native == "" {
		resolved, err := resolveHostPath(e.view, name)
		if err != nil {
			e.say(true, "'%s' %s", name, causeText(err))
			return false
		}
		native = resolved
	}
	if own && e.file.readOnly && !force {
		e.say(true, "'%s' is read only", name)
		return false
	}
	if _, err := os.Stat(native); !own && !force && !errors.Is(err, fs.ErrNotExist) {
		e.say(true, "File exists (:w! overrides)")
		return false
	}
	size, err := viWrite(e.view, native, e.lines, e.file.crlf)
	if err != nil {
		e.say(true, "'%s' %s", name, causeText(err))
		return false
	}
	e.say(false, "'%s' %dL, %dC", name, len(e.lines), size)
	if e.file.name == "" {
		e.file.name, e.file.native = name, native
		own = true
	}
	if own {
		e.markSaved()
	}
	return true
}
