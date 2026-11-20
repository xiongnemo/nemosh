package applets

// Indenting in an insert, busybox's FEATURE_VI_SETOPTS: with autoindent a new line -- Enter, o, O,
// cc -- begins with the indent of the line it came from, an Escape takes that indent off a line
// left with nothing else, and an Enter on such a line moves it onto the next; ^D takes the indent
// back to the tabstop before; and with expandtab a Tab is the spaces to the next tabstop.

// enterInsert begins an insert, or with R a replace, where the cursor is.
func (e *viEditor) enterInsert(mode viMode) {
	e.mode, e.insertStart = mode, e.cursor()
	e.replaceStart, e.replaced = e.col, nil
}

// indentOf is the blanks a line begins with.
func indentOf(line []rune) []rune {
	end := 0
	for end < len(line) && isViBlank(line[end]) {
		end++
	}
	return line[:end]
}

// giveIndent gives line row the indent from, when autoindent is set, and puts the cursor after
// it.
func (e *viEditor) giveIndent(row int, from []rune) {
	indent := append([]rune(nil), indentOf(from)...)
	if !e.autoIndent || len(indent) == 0 {
		return
	}
	e.replaceLines(row, row+1, [][]rune{append(indent, e.lines[row]...)})
	e.row, e.col = row, len(indent)
	e.indentRow, e.indentLen = row, len(indent)
}

// onlyAutoIndent says the cursor's line holds the indent autoindent gave it and nothing else.
func (e *viEditor) onlyAutoIndent() bool {
	line := e.line()
	return e.autoIndent && e.indentRow == e.row && e.indentLen == len(line) && len(indentOf(line)) == len(line) && len(line) > 0
}

// dropAutoIndent takes the indent off a line that has nothing else, as busybox's Escape does.
func (e *viEditor) dropAutoIndent() {
	if e.onlyAutoIndent() {
		e.replaceLines(e.row, e.row+1, [][]rune{{}})
		e.col = 0
	}
}

// outdent is ^D: the indent back to the tabstop before its end.
func (e *viEditor) outdent() {
	line := e.line()
	indent := len(indentOf(line))
	width := e.displayColumn(line, indent)
	target := width - e.tabstop
	if width%e.tabstop != 0 {
		target = width - width%e.tabstop
	}
	for indent > 0 && e.displayColumn(line, indent) > max(0, target) {
		indent--
		line = append(append([]rune(nil), line[:indent]...), line[indent+1:]...)
		e.col = max(0, e.col-1)
	}
	e.replaceLines(e.row, e.row+1, [][]rune{line})
	if e.indentRow == e.row {
		e.indentLen = len(indentOf(line))
	}
}

// insertTab is Tab: a tab, or with expandtab the spaces to the next tabstop.
func (e *viEditor) insertTab() {
	if !e.expandTab {
		e.insertRune('\t')
		return
	}
	column := e.displayColumn(e.line(), e.col)
	for range e.tabstop - column%e.tabstop {
		e.insertRune(' ')
	}
}
