import io

# --- The model: matching shared with the filter, and finding the next match. ---
p = 'internal/applets/top_model.go'
s = io.open(p, encoding='utf-8').read()

old = '''// matches is the filter: a substring of the name or of the command line, case-insensitively.
//
// The command line is included because it is the only way to tell four `svchost.exe` apart, and
// telling them apart is most of why anyone filters on Windows.
func (m topModel) matches(row topRow) bool {
	if m.Filter == "" {
		return true
	}
	needle := strings.ToLower(m.Filter)
	if strings.Contains(strings.ToLower(row.Process.Name), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(row.Details.CommandLine), needle) {
		return true
	}
	// A pid typed into the filter should find that process, which is what someone who has a
	// number from somewhere else will try first.
	return strconv.Itoa(row.Process.PID) == m.Filter
}'''
new = '''// matches is the filter: a substring of the name or of the command line, case-insensitively.
func (m topModel) matches(row topRow) bool {
	if m.Filter == "" {
		return true
	}
	return rowMatches(row, strings.ToLower(m.Filter))
}

// rowMatches is what counts as a match, for the filter and the search alike.
//
// One function for both, because htop keeps them as separate operations -- a filter hides what does
// not match, a search jumps to what does -- and two operations that disagree about what a match *is*
// would be a trap: typing the same word into each would find different processes.
//
// The command line is included because it is the only way to tell four `svchost.exe` apart, and
// telling them apart is most of why anyone searches on Windows. needle must already be lowercase.
func rowMatches(row topRow, needle string) bool {
	if strings.Contains(strings.ToLower(row.Process.Name), needle) {
		return true
	}
	if strings.Contains(strings.ToLower(row.Details.CommandLine), needle) {
		return true
	}
	// A pid typed in should find that process, which is what someone who has a number from
	// somewhere else will try first.
	return strconv.Itoa(row.Process.PID) == needle
}

// findTopMatch is the next row matching term, starting after the given index and wrapping.
//
// Wrapping, and starting *after* rather than at, because that is what makes repeated presses walk
// through the matches instead of sticking on the first one. Pass -1 to start from the top, which is
// what an incremental search does on every keystroke.
func findTopMatch(rows []topRow, term string, after int) (int, bool) {
	if term == "" || len(rows) == 0 {
		return 0, false
	}
	needle := strings.ToLower(term)
	for offset := 1; offset <= len(rows); offset++ {
		index := ((after+offset)%len(rows) + len(rows)) % len(rows)
		if rowMatches(rows[index], needle) {
			return index, true
		}
	}
	return 0, false
}'''
assert old in s
s = s.replace(old, new, 1)

old = '''	// Filter narrows the list to names and command lines containing it, case-insensitively.
	Filter string'''
new = '''	// Filter narrows the list to names and command lines containing it, case-insensitively.
	Filter string
	// Search is the last thing searched for, which is what makes a repeat key possible: F3 and
	// n walk to the next match without asking again. Searching leaves the list whole, unlike
	// the filter -- htop is right to separate them, and this had them conflated.
	Search string'''
assert old in s
s = s.replace(old, new, 1)

old = '''	topActionSearchPrompt
	topActionFilterPrompt
	topActionRefresh'''
new = '''	topActionSearchPrompt
	// topActionSearchNext walks to the match after the cursor, which needs the drawn row list
	// and so cannot be done by the model alone.
	topActionSearchNext
	topActionFilterPrompt
	topActionRefresh'''
assert old in s
s = s.replace(old, new, 1)

old = '''	case "F3", "/":
		return topActionSearchPrompt'''
new = '''	case "/":
		return topActionSearchPrompt
	case "F3":
		// htop's F3 is "search next" once there is something to search for, and asking
		// again for a term you have already typed is the wrong answer to that key.
		if m.Search == "" {
			return topActionSearchPrompt
		}
		return topActionSearchNext
	case "n":
		return topActionSearchNext'''
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)

# --- The view: a prompt in the footer, and the input capture standing aside for it. ---
p = 'internal/applets/top_keys.go'
s = io.open(p, encoding='utf-8').read()

old = '''func (v *topView) key(event *tcell.EventKey) *tcell.EventKey {
	name := topKeyName(event)
	if name == "" {
		return event
	}'''
new = '''func (v *topView) key(event *tcell.EventKey) *tcell.EventKey {
	if v.prompting {
		// Something else owns the keyboard -- a prompt, or the kill confirmation. This
		// capture is on the *application*, so it sees every key before any widget does, and
		// without this a filter typed into an input field was read as commands: the `q` in
		// `sqlservr` quit the program.
		return event
	}
	name := topKeyName(event)
	if name == "" {
		return event
	}'''
assert old in s
s = s.replace(old, new, 1)

old = '''	case topActionSearchPrompt:
		// Searching is not filtering, and until it can jump to a match it says so rather
		// than quietly doing the other thing -- which is what it did before reading how
		// htop separates the two.
		v.status.SetText("[yellow]search jumps to a match and is not wired yet; F4 filters instead")'''
new = '''	case topActionSearchPrompt:
		v.promptSearch()
	case topActionSearchNext:
		v.searchNext()'''
assert old in s
s = s.replace(old, new, 1)

old = '''// promptFilter takes a filter interactively.
func (v *topView) promptFilter() {
	field := tview.NewInputField().SetLabel("filter: ").SetText(v.session.model.Filter)
	field.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			v.session.model.Filter = field.GetText()
		}
		v.application.SetRoot(v.root, true)
		v.refresh()
	})
	v.application.SetRoot(field, true)
}'''
new = '''// promptFilter takes a filter interactively.
func (v *topView) promptFilter() {
	before := v.session.model.Filter
	field := tview.NewInputField().SetLabel("filter: ").SetText(before)
	field.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			v.session.model.Filter = field.GetText()
		} else {
			v.session.model.Filter = before
		}
		v.hidePrompt(field)
		v.refresh()
	})
	v.showPrompt(field)
}

// promptSearch searches as it is typed, leaving the list whole.
//
// Incremental, because that is what makes a search worth having over a filter: the table stays
// where it is and the cursor walks to the match, so the process is found *in context* -- with its
// neighbours, its parent, and its share of the machine still on screen.
func (v *topView) promptSearch() {
	before := v.session.model.Selected
	field := tview.NewInputField().SetLabel("search: ").SetText(v.session.model.Search)
	field.SetChangedFunc(func(text string) {
		v.session.model.Search = text
		// From the top on every keystroke, so the answer depends on what was typed and
		// not on how it was typed. Searching onwards from the cursor as it grew would walk
		// forward through the table on every letter.
		v.moveToMatch(findTopMatch(v.rows, text, -1))
	})
	field.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			// Escape puts the cursor back where it was, which is what makes trying a
			// search cost nothing.
			v.session.model.Selected = before
			v.fillTable()
		}
		v.hidePrompt(field)
		v.refresh()
	})
	v.showPrompt(field)
}

// searchNext walks to the match after the cursor.
func (v *topView) searchNext() {
	term := v.session.model.Search
	if term == "" {
		v.promptSearch()
		return
	}
	row, _ := v.table.GetSelection()
	if !v.moveToMatch(findTopMatch(v.rows, term, row-1)) {
		v.status.SetText(fmt.Sprintf("[yellow]no process matches %q", term))
	}
}

// moveToMatch puts the cursor on a found row, and says so when there was none.
func (v *topView) moveToMatch(index int, found bool) bool {
	if !found {
		return false
	}
	v.session.model.Selected = v.rows[index].Process.PID
	v.table.Select(index+1, 0)
	return true
}

// showPrompt puts an input field where the status line is.
//
// In the footer rather than over the whole screen, which is how this started: SetRoot on the field
// replaced the table with a single line, so a filter or a search was typed blind. The point of a
// search is to see where the cursor went.
func (v *topView) showPrompt(field *tview.InputField) {
	v.prompting = true
	v.root.RemoveItem(v.status)
	v.root.AddItem(field, 1, 0, true)
	v.application.SetFocus(field)
}

// hidePrompt puts the status line back.
func (v *topView) hidePrompt(field *tview.InputField) {
	v.prompting = false
	v.root.RemoveItem(field)
	v.root.AddItem(v.status, 1, 0, false)
	v.application.SetFocus(v.table)
}'''
assert old in s
s = s.replace(old, new, 1)

old = '''		modal.SetDoneFunc(func(index int, label string) {'''
new = '''		modal.SetDoneFunc(func(index int, label string) {
			v.prompting = false'''
assert old in s
s = s.replace(old, new, 1)
old = '''	v.application.SetRoot(modal, true)
}'''
new = '''	v.prompting = true
	v.application.SetRoot(modal, true)
}'''
assert old in s
s = s.replace(old, new, 1)

old = '''const topHelpText = "[white]q quit  F4 filter  F5/t tree  H threads  K kernel  I reverse  " +
	"P/M/T/N sort  space tag  +/- fold  Z pause  p path  F9/k kill  r refresh"'''
new = '''const topHelpText = "[white]q quit  / search  n next  F4 filter  F5/t tree  H threads  K kernel  " +
	"I reverse  P/M/T/N sort  space tag  +/- fold  Z pause  p path  F9/k kill  r refresh"'''
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)

# --- The field, and the search shown in the status line. ---
p = 'internal/applets/top_view.go'
s = io.open(p, encoding='utf-8').read()
old = '''	// filling is set while the table is being rebuilt, so the selection callback ignores the
	// row numbers it sees in the middle of that.
	filling bool'''
new = '''	// filling is set while the table is being rebuilt, so the selection callback ignores the
	// row numbers it sees in the middle of that.
	filling bool
	// prompting is set while a prompt or a modal owns the keyboard, so the application's input
	// capture lets keys through to it instead of reading them as commands.
	prompting bool'''
assert old in s
s = s.replace(old, new, 1)

old = '''	filter := ""
	if model.Filter != "" {
		filter = fmt.Sprintf("  filter=%q", model.Filter)
	}'''
new = '''	filter := ""
	if model.Filter != "" {
		filter = fmt.Sprintf("  filter=%q", model.Filter)
	}
	if model.Search != "" {
		// Shown because it is what `n` will act on, and a repeat key whose target is
		// invisible is a repeat key nobody presses.
		filter += fmt.Sprintf("  search=%q", model.Search)
	}'''
assert old in s
s = s.replace(old, new, 1)
io.open(p, 'w', encoding='utf-8', newline='\n').write(s)
print("search written")
