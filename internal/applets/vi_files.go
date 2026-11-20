package applets

import (
	"os"
	"strings"
)

// Files, busybox's: vi FILE... edits them in turn, :n and :prev move between them -- :N too,
// vim's, busybox's :p being vim's print -- and :rew goes back to the first; :q with files left
// says how many, and :q! leaves them all. :e reads another, or the same again with !, # the one
// before; :f renames the buffer; :r reads a file in after a line, :0r before the first.

// remaining is how many files are left after this one.
func (e *viEditor) remaining() int { return max(0, len(e.files)-e.fileIndex-1) }

// editFile reads name into the buffer in place of what it held, busybox's init_text_buffer,
// and says what it read: `'f' 3L, 14C`, `[New file]` for one not there.
func (e *viEditor) editFile(name string) {
	if name != e.file.name && e.file.name != "" {
		e.alternate = e.file.name
	}
	file, lines, message := viLoad(e.view, name, e.readOnlyMode)
	file.crlf = file.crlf && !e.binary
	e.file, e.lines, e.savedCRLF = file, lines, file.crlf
	e.undo, e.redo, e.seq, e.savedSeq, e.changeOpen = nil, nil, 0, 0, false
	e.marks, e.context, e.lineBefore = [26]viMark{}, viMark{}, nil
	e.top, e.left = 0, 0
	e.goToStartLine(0)
	if message != "" {
		e.say(true, "%s", message)
		return
	}
	flags := ""
	if file.isNew {
		flags += " [New file]"
	}
	if file.readOnly {
		flags += " [Readonly]"
	}
	e.say(false, "'%s'%s %dL, %dC", name, flags, len(lines), viChars(lines))
}

// viChars is busybox's end - text: each line's characters and its newline.
func viChars(lines [][]rune) int {
	chars := 0
	for _, line := range lines {
		chars += len(line) + 1
	}
	return chars
}

// exEdit is :e [FILE] and :e!.
func (e *viEditor) exEdit(name string, force bool, argument string) {
	if e.isModified() && !force {
		e.say(true, "No write since last change (:%s! overrides)", name)
		return
	}
	switch {
	case argument == "#" && e.alternate == "":
		e.say(true, "No previous filename")
	case argument == "#":
		e.editFile(e.alternate)
	case argument != "":
		e.editFile(argument)
	case e.file.name == "":
		e.say(true, "No current filename")
	default:
		e.editFile(e.file.name)
	}
}

// exSwitch is :n, :N, :prev and :rew: the next file, the one before, or the first.
func (e *viEditor) exSwitch(command, name string, force bool) {
	if e.isModified() && !force {
		e.say(true, "No write since last change (:%s! overrides)", name)
		return
	}
	index := e.fileIndex
	switch command {
	case "next", "wnext":
		if e.remaining() == 0 {
			e.say(true, "No more files to edit")
			return
		}
		index++
	case "rewind":
		index = 0
	default:
		if index == 0 {
			e.say(true, "No previous files to edit")
			return
		}
		index--
	}
	e.fileIndex = index
	e.editFile(e.files[index])
}

// exFile is :f [NAME]: the buffer is called NAME from now on, and the status line says so.
func (e *viEditor) exFile(given int, argument string) {
	if given > 0 {
		e.say(true, "No address allowed on this command")
		return
	}
	if argument != "" {
		native, err := resolveHostPath(e.view, argument)
		if err != nil {
			e.say(true, "'%s' %s", argument, causeText(err))
			return
		}
		e.file.name, e.file.native = argument, native
	}
	e.say(false, "%s", e.status())
}

// exRead is :r FILE after line, 0 being before the first; :r !COMMAND reads what it writes.
func (e *viEditor) exRead(line int, argument string) {
	var lines [][]rune
	name := argument
	if command, ok := strings.CutPrefix(argument, "!"); ok {
		lines = e.commandOutput(command)
		if lines == nil {
			return
		}
	} else {
		if name == "" {
			name = e.file.name
		}
		if name == "" {
			e.say(true, "No current filename")
			return
		}
		native, err := resolveHostPath(e.view, name)
		if err == nil {
			var handle *os.File
			if handle, err = os.Open(native); err == nil {
				var text []string
				text, _, _, err = readEdLines(handle)
				handle.Close()
				for _, piece := range text {
					lines = append(lines, []rune(piece))
				}
			}
		}
		if err != nil {
			e.say(true, "'%s' %s", name, causeText(err))
			return
		}
		flags := ""
		if e.file.readOnly {
			flags = " [Readonly]"
		}
		e.say(false, "'%s'%s %dL, %dC", name, flags, len(lines), viChars(lines))
	}
	if len(lines) == 0 {
		return
	}
	e.replaceLines(line, line, lines)
	e.row, e.col = line, e.firstNonBlank(line)
}
