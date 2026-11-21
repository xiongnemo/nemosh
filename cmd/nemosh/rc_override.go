package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A session in bash mode reads bash's startup file and then nemosh's, the user's decision of
// 2026-11-21, and nemosh's has the last word: a name it changes of what bash's set is said, on
// one line of stderr, rather than taken without a word. Names are compared as runtime's
// Definitions gives them: PS1, alias ll, function f, set -o vi, shopt nullglob, complete git.

// overridden names, in order, what the second file changed of what the first set: changed from
// before to afterFirst, and again from beforeSecond to after. A variable the second only added
// to, whose new value holds the old one -- PATH=$PATH:/more -- is not counted.
func overridden(before, afterFirst, beforeSecond, after runtime.Definitions) []string {
	var names []string
	for name, set := range afterFirst {
		if old, had := before[name]; had && old == set {
			continue
		}
		now, kept := after[name]
		was := beforeSecond[name]
		if kept && now == was {
			continue
		}
		if kept && was != "" && !strings.Contains(name, " ") && strings.Contains(now, was) {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// overriddenNotice is the line that says so, the first five names and how many more.
func overriddenNotice(second, first string, names []string) string {
	shown := names
	more := ""
	if len(names) > 5 {
		shown, more = names[:5], fmt.Sprintf(", and %d more", len(names)-5)
	}
	return fmt.Sprintf("nemosh: %s changed what %s set: %s%s\n", second, first, strings.Join(shown, ", "), more)
}

// nemoshRCName is nemosh's rc file as a person would name it: $ENV as given, or ~/.nemoshrc,
// and false when an empty ENV asks for none.
func nemoshRCName(rt runtime.Runtime) (string, bool) {
	path, present := rt.LookupVariable("ENV")
	if !present || path == "" {
		if value, ok := rt.LookupEnv("ENV"); ok {
			path, present = value, true
		}
	}
	switch {
	case !present:
		return "~/.nemoshrc", true
	case path == "":
		return "", false
	}
	return path, true
}

// bashFileName is the bash startup file the session read, as a person would name it: a login
// one's profile, --rcfile's, or ~/.bashrc.
func (c command) bashFileName(rt runtime.Runtime) string {
	switch {
	case c.invocation.login:
		for _, name := range [...]string{".bash_profile", ".bash_login"} {
			if homeFileExists(rt, name) {
				return "~/" + name
			}
		}
		return "~/.profile"
	case c.invocation.rcfile != "":
		return c.invocation.rcfile
	}
	return "~/.bashrc"
}
