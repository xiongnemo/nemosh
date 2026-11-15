package applets

import (
	"path"
	"strings"
)

// grep's long options: GNU's, which busybox's grep does not have.
//
// `grep -rn --include='*.go' foo .` is the line people type after years of GNU grep, and it was
// `unknown option -- include=*.go`. busybox-w32 refuses it too, so these are an extension past the
// reference rather than a correction to match it, measured against GNU grep: --include,
// --exclude and --exclude-dir, which name what a search reads; -a and -I, which say what to do
// with a binary file (grep_binary.go); and --color, which now colours (grep_color.go) where it
// was accepted and did nothing.

// grepNameFilters are the globs that choose what -r reads, each matched against a base name as
// GNU matches them: --include and --exclude against a file's, --exclude-dir against a
// directory's. Several of one are any of them.
type grepNameFilters struct {
	include    []string
	exclude    []string
	excludeDir []string
}

// takesFile reports whether a file of this base name is read: not excluded, and included when
// any --include was given.
func (f grepNameFilters) takesFile(base string) bool {
	if matchesAnyGlob(f.exclude, base) {
		return false
	}
	return len(f.include) == 0 || matchesAnyGlob(f.include, base)
}

// takesDirectory reports whether -r goes into a directory of this base name.
func (f grepNameFilters) takesDirectory(base string) bool {
	return !matchesAnyGlob(f.excludeDir, base)
}

func matchesAnyGlob(globs []string, base string) bool {
	for _, glob := range globs {
		if matched, err := path.Match(glob, base); err == nil && matched {
			return true
		}
	}
	return false
}

// parseGrepLongOption reads one long option, and the next argument when it is the option's
// value and was not joined to it with `=`, as getopt_long takes either. It answers how many
// arguments it used.
func parseGrepLongOption(args []string, index int, flags *grepFlags) (int, error) {
	arg := args[index]
	name, value, present := strings.Cut(arg[2:], "=")
	switch name {
	case "color", "colour":
		// An absent value is always, as GNU and busybox read it; a separate word is not its
		// value, since --color takes an optional argument. --colour is GNU's other spelling,
		// accepted and not offered: offered, --col and Tab stopped at --colo.
		when, err := parseColorWhen(value, present)
		flags.color = when
		return 1, err
	case "text":
		flags.binary = grepBinaryText
		return 1, nil
	case "include", "exclude", "exclude-dir":
	default:
		return 0, unknownLongOption(arg)
	}
	used := 1
	if !present {
		if index+1 >= len(args) {
			return 0, missingOptionArgument(name)
		}
		value, used = args[index+1], 2
	}
	switch name {
	case "include":
		flags.names.include = append(flags.names.include, value)
	case "exclude":
		flags.names.exclude = append(flags.names.exclude, value)
	default:
		flags.names.excludeDir = append(flags.names.excludeDir, value)
	}
	return used, nil
}
