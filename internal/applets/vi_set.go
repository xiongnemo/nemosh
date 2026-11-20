package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// :set, busybox's FEATURE_VI_SETOPTS -- autoindent, binary, expandtab, fileformat, fileformats,
// flash, ignorecase, showmatch, tabstop, each by its short name too -- and vim's number. A
// boolean is set by its name and cleared by no and its name; a value follows =; vim's opt? says
// what an option is. :set and :set all list them, as busybox does.

// viOption is one option: its names, and the setting it is.
type viOption struct {
	name, short string
	flag        func(e *viEditor) *bool
}

var viOptions = [...]viOption{
	{"autoindent", "ai", func(e *viEditor) *bool { return &e.autoIndent }},
	{"binary", "bin", func(e *viEditor) *bool { return &e.binary }},
	{"expandtab", "et", func(e *viEditor) *bool { return &e.expandTab }},
	{"fileformat", "ff", nil},
	{"fileformats", "ffs", nil},
	{"flash", "fl", func(e *viEditor) *bool { return &e.flash }},
	{"ignorecase", "ic", func(e *viEditor) *bool { return &e.ignoreCase }},
	{"number", "nu", func(e *viEditor) *bool { return &e.number }},
	{"showmatch", "sm", func(e *viEditor) *bool { return &e.showMatch }},
	{"tabstop", "ts", nil},
}

func lookupViOption(name string) (viOption, bool) {
	for _, option := range viOptions {
		if name == option.name || name == option.short {
			return option, true
		}
	}
	return viOption{}, false
}

func (e *viEditor) setOptions(argument string) {
	if argument == "" || argument == "all" {
		e.showLines(e.optionListing())
		return
	}
	for _, word := range strings.Fields(argument) {
		if !e.setOption(word) {
			e.say(true, "bad option: %s", word)
			return
		}
	}
}

// setOption is one word of :set, and says whether it was one.
func (e *viEditor) setOption(word string) bool {
	name, value, valued := strings.Cut(word, "=")
	if asked, ok := strings.CutSuffix(name, "?"); ok && !valued {
		option, known := lookupViOption(asked)
		if known {
			e.say(false, "%s", e.optionText(option))
		}
		return known
	}
	option, known := lookupViOption(name)
	negated := false
	if !known && strings.HasPrefix(name, "no") {
		option, known = lookupViOption(name[2:])
		negated = true
	}
	switch {
	case !known:
		return false
	case option.flag != nil:
		if valued {
			return false
		}
		*option.flag(e) = !negated
		return true
	case negated || !valued:
		return false
	case option.name == "tabstop":
		stop, err := strconv.Atoi(value)
		if err != nil || stop < 1 || stop > 32 {
			return false
		}
		e.tabstop = stop
	case option.name == "fileformat":
		if value != "dos" && value != "unix" {
			return false
		}
		e.file.crlf = value == "dos"
	default:
		if value != "unix,dos" && value != "dos,unix" && value != "unix" && value != "dos" {
			return false
		}
		e.fileFormats = value
	}
	return true
}

func (e *viEditor) optionText(option viOption) string {
	switch {
	case option.flag != nil && *option.flag(e):
		return option.name
	case option.flag != nil:
		return "no" + option.name
	case option.name == "tabstop":
		return fmt.Sprintf("tabstop=%d", e.tabstop)
	case option.name == "fileformat" && e.file.crlf:
		return "fileformat=dos"
	case option.name == "fileformat":
		return "fileformat=unix"
	}
	return "fileformats=" + e.fileFormats
}

// optionListing is busybox's :set all, its two lines.
func (e *viEditor) optionListing() []string {
	var words []string
	for _, option := range viOptions {
		words = append(words, e.optionText(option))
	}
	return []string{strings.Join(words[:5], " "), strings.Join(words[5:], " ")}
}
