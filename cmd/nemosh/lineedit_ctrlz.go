package main

// Ctrl-Z.
//
// busybox-w32's editor turns every backslash on the line into a slash (libbb/lineedit.c,
// CTRL('Z'), bs_to_slash), which is what a Windows path pasted at the prompt needs: the shell
// reads C:\Users\nemo as C:Usersnemo, and C:/Users/nemo as the path. This shell read Ctrl-Z as
// end of input, as cmd does, so a stray one -- suspend, to anyone used to a Unix shell --
// closed the session. It now does what busybox's does, and NEMOSH_CTRL_Z=eof is the way back.

// ctrlZ handles the key, and reports whether it ended input. Under NEMOSH_CTRL_Z=eof it is
// Ctrl-D: end of input on an empty line, and a forward delete on one with text, which would
// otherwise be thrown away.
func (e *lineEditor) ctrlZ() bool {
	if !e.ctrlZEndsInput() {
		e.buffer.slashBackslashes()
		return false
	}
	if e.buffer.isEmpty() {
		return true
	}
	e.buffer.deleteForward()
	return false
}

// ctrlZEndsInput reports whether NEMOSH_CTRL_Z asks for Ctrl-Z to end input. Asked at each
// press rather than once, so setting it in an rc file, or at the prompt, takes effect at once.
func (e *lineEditor) ctrlZEndsInput() bool {
	if e.setting == nil {
		return false
	}
	value, _ := e.setting("NEMOSH_CTRL_Z")
	return value == "eof"
}

// slashBackslashes turns the line's backslashes into slashes, leaving the cursor where it was:
// one character for one, so its position still names the same place in the line.
func (b *lineBuffer) slashBackslashes() {
	for index, r := range b.runes {
		if r == '\\' {
			b.runes[index] = '/'
		}
	}
}
