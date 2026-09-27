package runtime

import (
	"os"
	"strconv"
	"strings"
)

// setStartupVariables sets what a new shell sets whatever it inherited, as busybox ash's init
// does (shell/ash.c): OPTIND is 1, so a getopts loop starts at the first argument, and an
// inherited OPTIND stays exported with that value; PS4 is `+ ` unless the environment gave
// one; and HOSTNAME is the computer's name when nothing set it, a shell variable that is not
// exported. None of the three was set.
func (r Runtime) setStartupVariables() {
	r.vars["OPTIND"] = "1"
	if r.isExported("OPTIND") {
		r.env.Set("OPTIND", "1")
	}
	if _, set := r.vars["PS4"]; !set {
		r.vars["PS4"] = "+ "
	}
	if _, set := r.vars["HOSTNAME"]; !set {
		if name, err := os.Hostname(); err == nil {
			r.vars["HOSTNAME"] = name
		}
	}
}

// EnterShellLevel is a shell's start: SHLVL goes up by one and is exported, as in both
// references, so a script can tell how deeply it was started. A SHLVL that is not a number
// counts as none, as busybox counts it, and a level below zero is zero, as bash has it. A
// subshell or a background job is the same shell, and does not count.
func (r Runtime) EnterShellLevel() {
	text := r.vars["SHLVL"]
	level := atoiOrZero(strings.TrimPrefix(text, "-"))
	if strings.HasPrefix(text, "-") {
		level = -level
	}
	r.vars["SHLVL"] = strconv.Itoa(max(level+1, 0))
	r.env.Set("SHLVL", r.vars["SHLVL"])
}
