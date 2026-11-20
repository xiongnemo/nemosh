package applets

import (
	"errors"
	"io/fs"
	"os"
)

// The ex commands that write and quit.

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
