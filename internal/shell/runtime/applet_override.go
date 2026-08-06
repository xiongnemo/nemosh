package runtime

import (
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// overrideAppletsVariable is Nemosh's spelling of busybox-w32's
// BB_OVERRIDE_APPLETS (README.md:41), which lets a user prefer an external
// program to a bundled applet of the same name. The value is read from the
// shell's own variable table, the way PATH already is
// (external.go:103). busybox has to reach for getenv because prefer_applet
// lives in libbb rather than in ash, and pays for it: ash mirrors the variable
// into the real environment on export (shell/ash.c:2976) so that libbb can see
// it, which means an unexported assignment silently does nothing there. Nemosh
// has no such split, so a plain assignment takes effect.
const overrideAppletsVariable = "NEMOSH_OVERRIDE_APPLETS"

// overrideSeparators are busybox's exactly (libbb/appletlib.c:321). A tab is
// not one of them.
const overrideSeparators = " ,;"

// appletPreferred reports whether the bundled applet named name still wins over
// an external program of the same name.
//
// The override is scoped to shell lookup, not to the multi-call binary:
// `nemosh cat` and a `cat` shim stay unconditional. busybox routes even its
// multi-call entry through the check (libbb/appletlib.c:279), which makes an
// override of a name with no external counterpart -- the usual case for a shim
// -- fail outright. docs/design/windows-execution-model.md:38 scopes the
// override to "shell standalone lookup" and lists the other two forms without
// qualification, so Nemosh follows the doc here.
func (r Runtime) appletPreferred(name string) bool {
	return preferApplet(name, r.vars[overrideAppletsVariable], r.externalExists)
}

// lookupApplet is the shell's applet lookup, and the only one the shell should
// use: the registry, gated by the override. Reporting an overridden applet as
// absent is what makes the rest of the lookup fall through to PATH on its own,
// and it keeps `command -v` telling the truth about what would run -- one
// routine answers both, as find_command does in busybox (shell/ash.c:9861).
func (r Runtime) lookupApplet(name string) (applets.Applet, bool) {
	applet, found := r.registry.Lookup(name)
	if !found || !r.appletPreferred(name) {
		return nil, false
	}
	return applet, true
}

func (r Runtime) externalExists(name string) bool {
	_, err := r.externalCommandPath(name)
	return err == nil
}

// preferApplet is busybox's prefer_applet_internal (libbb/appletlib.c:296).
func preferApplet(name, override string, externalExists func(string) bool) bool {
	switch override {
	case "":
		return true
	case "-":
		// Every applet is disabled.
		return false
	case "+":
		// Every applet is disabled that an external can stand in for.
		return !externalExists(name)
	}
	listed, conditional := overrideLists(name, override)
	if !listed {
		return true
	}
	if !conditional {
		return false
	}
	return !externalExists(name)
}

// overrideLists answers whether the list names the applet and, if so, whether
// the name sits after the first semicolon, which is what makes its override
// conditional on an external existing. busybox stops at the first bounded
// occurrence, so a name written on both sides of the semicolon takes the
// unconditional reading.
func overrideLists(name, override string) (listed, conditional bool) {
	unconditional, rest, split := strings.Cut(override, ";")
	if overrideNames(unconditional, name) {
		return true, false
	}
	return split && overrideNames(rest, name), true
}

func overrideNames(list, name string) bool {
	for _, entry := range strings.FieldsFunc(list, isOverrideSeparator) {
		if entry == name {
			return true
		}
	}
	return false
}

func isOverrideSeparator(r rune) bool {
	return strings.ContainsRune(overrideSeparators, r)
}
