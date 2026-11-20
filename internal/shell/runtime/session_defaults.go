package runtime

// A session starts as Ubuntu's does, whose ~/.bashrc, copied from /etc/skel, gives every account
// ll, la and l, and ls and the greps in colour on a terminal: what a Debian or Ubuntu
// administrator types without thinking, and found `not found` here, or ls in no colour. Only a
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

// SetSessionDefaults gives a session what Ubuntu's gets, before its rc file is read.
func (r Runtime) SetSessionDefaults() {
	if r.vars[sessionDefaultsVariable] == "off" {
		return
	}
	for _, alias := range ubuntuAliases {
		r.aliases[alias[0]] = alias[1]
	}
}
