package runtime

import "strconv"

// A session starts as Ubuntu's does, whose ~/.bashrc, copied from /etc/skel, gives every account
// ll, la and l, ls and the greps in colour on a terminal, and dircolors' LS_COLORS: what a Debian
// or Ubuntu administrator types without thinking, and found `not found` here, or ls in no colour,
// or in busybox's three, which colour an archive as any other file. Only a
// session has them, as only a session reads ~/.bashrc, so a script is as it was; the rc file
// comes after, and can take any away. NEMOSH_DEFAULTS=off in the environment is a session
// without them, as one was until 2026-11-20.

// sessionDefaultsVariable turns the defaults off when it says off.
const sessionDefaultsVariable = "NEMOSH_DEFAULTS"

// ubuntuAliases are Ubuntu's ~/.bashrc's, in its order.
var ubuntuAliases = [...][2]string{
	{"ls", "ls --color=auto"},
	{"grep", "grep --color=auto"},
	{"fgrep", "fgrep --color=auto"},
	{"egrep", "egrep --color=auto"},
	{"ll", "ls -alF"},
	{"la", "ls -A"},
	{"l", "ls -CF"},
}

// NoteWindowSize is bash's checkwinsize, on in a new shell: a session sets COLUMNS and LINES to
// the terminal's size before each prompt, so a prompt and what is run from one can read it. A
// readonly one is left, and an exported one goes on being, with the new value. `shopt -u
// checkwinsize` leaves both as they are.
func (r Runtime) NoteWindowSize(columns, lines int) {
	if columns <= 0 || lines <= 0 || !r.options.checkWinSize {
		return
	}
	for _, size := range [...]struct {
		name  string
		value int
	}{{"COLUMNS", columns}, {"LINES", lines}} {
		if _, readonly := r.readonly[size.name]; readonly {
			continue
		}
		r.vars[size.name] = strconv.Itoa(size.value)
		if r.isExported(size.name) {
			r.env.Set(size.name, r.vars[size.name])
		}
	}
}

// SetSessionDefaults gives a session what Ubuntu's gets, before its rc file is read.
func (r Runtime) SetSessionDefaults() {
	if r.vars[sessionDefaultsVariable] == "off" {
		return
	}
	for _, alias := range ubuntuAliases {
		r.aliases[alias[0]] = alias[1]
	}
	// And LS_COLORS, exported, as `eval "$(dircolors -b)"` leaves it; one already set is kept,
	// as Ubuntu's ~/.dircolors would be read in place of the database.
	if _, set := r.vars["LS_COLORS"]; !set {
		r.vars["LS_COLORS"] = dircolorsDatabase
		r.markExported("LS_COLORS")
	}
}
