package runtime

import (
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// setStartupVariables sets what a new shell sets whatever it inherited, as busybox ash's init
// does (shell/ash.c): OPTIND is 1, so a getopts loop starts at the first argument, and an
// inherited OPTIND stays exported with that value; PS4 is `+ ` unless the environment gave
// one; and HOSTNAME is the computer's name when nothing set it, a shell variable that is not
// exported. None of the three was set. bash's UID, EUID and OSTYPE follow; see
// setIdentityVariables.
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
	r.setIdentityVariables()
}

// setIdentityVariables sets bash's UID and EUID, integers and read-only as there, and OSTYPE,
// none of which busybox has, so bash decides. UID is the user `id -u` answers for -- on Windows
// 0 only when the shell is elevated, and busybox-w32's 4095 otherwise -- and EUID the one the
// shell acts as, which on Windows is the same token. A script that asks `[ "$EUID" -ne 0 ]`
// before it needs root was comparing nothing, an error that read as false. OSTYPE is msys on
// Windows, the value scripts test for Git Bash, which MSYS2's bash says and Git for Windows'
// said for years; its bash 2.55 says cygwin. Elsewhere it is linux-gnu, or the GOOS, darwin
// for macOS, whose bash adds a version a `darwin*` test passes over. An OSTYPE already set is
// kept, as bash keeps one.
func (r Runtime) setIdentityVariables() {
	uid := applets.CurrentUserID()
	euid := os.Geteuid()
	if euid < 0 {
		euid = uid
	}
	for name, value := range map[string]int{"UID": uid, "EUID": euid} {
		r.vars[name] = strconv.Itoa(value)
		r.readonly[name] = struct{}{}
		attributes := r.attributes[name]
		attributes.integer = true
		r.attributes[name] = attributes
	}
	if _, set := r.vars["OSTYPE"]; !set {
		r.vars["OSTYPE"] = operatingSystemType()
	}
}

// operatingSystemType is OSTYPE for the system this runs on, as bash names it.
func operatingSystemType() string {
	switch runtime.GOOS {
	case "windows":
		return "msys"
	case "linux":
		return "linux-gnu"
	}
	return runtime.GOOS
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
