package runtime

import (
	"fmt"
	"sort"
	"strings"
)

// `shopt` -- the option builtin bash keeps separate from `set -o`.
//
// It was not a builtin, so `shopt -s globstar` failed with `shopt: not found`, and
// the three glob options it is nearly always used for could not be turned on at all.
//
// Only the options this shell can honour are named. `shopt -s extglob` is refused
// rather than accepted, because accepting it would leave a script believing `@(a|b)`
// works -- and a pattern that silently matches nothing is the failure this whole pass
// has been about.

// shoptOption is one settable name.
type shoptOption struct {
	name  string
	field func(*shellOptions) *bool
	// why says what it does, for the listing.
	why string
}

var shoptOptions = []shoptOption{
	{"dotglob", func(o *shellOptions) *bool { return &o.dotGlob }, "a leading dot is matched by * as well"},
	{"globstar", func(o *shellOptions) *bool { return &o.globStar }, "** matches across directories"},
	{"nocaseglob", func(o *shellOptions) *bool { return &o.noCaseGlob }, "patterns match without regard to case"},
	{"nullglob", func(o *shellOptions) *bool { return &o.nullGlob }, "a pattern matching nothing expands to nothing"},
	{"extglob", func(o *shellOptions) *bool { return &o.extGlob }, "?() *() +() @() !() in patterns; always on here"},
}

func lookupShoptOption(name string) (shoptOption, bool) {
	for _, option := range shoptOptions {
		if option.name == name {
			return option, true
		}
	}
	return shoptOption{}, false
}

// shoptBuiltin is `shopt [-suqp] [name ...]`.
func (r Runtime) shoptBuiltin(args []string) int {
	set, unset, quiet, names, err := parseShoptArgs(args)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "shopt: %v\n", err)
		return 2
	}
	if len(names) == 0 {
		if set || unset {
			return r.listShoptOptions(set)
		}
		return r.listShoptOptions(false, true)
	}
	status := 0
	for _, name := range names {
		option, known := lookupShoptOption(name)
		if !known {
			fmt.Fprintf(r.streams.Stderr,
				"shopt: %s: not an option this build has; it has %s\n", name, strings.Join(shoptOptionNames(), ", "))
			status = 1
			continue
		}
		switch {
		case set:
			*option.field(r.options) = true
		case unset:
			// extglob cannot be turned off: the matcher recognises the operators
			// unconditionally, for the reason pattern_extended.go gives. Saying so beats
			// accepting the request and going on matching them.
			if name == "extglob" {
				fmt.Fprintln(r.streams.Stderr,
					"shopt: extglob is always on in this build and cannot be turned off")
				status = 1
				continue
			}
			*option.field(r.options) = false
		case quiet:
			if !*option.field(r.options) {
				status = 1
			}
		default:
			r.printShoptOption(option)
		}
	}
	return status
}

func parseShoptArgs(args []string) (set, unset, quiet bool, names []string, err error) {
	index := 0
	for ; index < len(args); index++ {
		argument := args[index]
		if argument == "--" {
			index++
			break
		}
		if len(argument) < 2 || argument[0] != '-' {
			break
		}
		for _, letter := range argument[1:] {
			switch letter {
			case 's':
				set = true
			case 'u':
				unset = true
			case 'q':
				quiet = true
			case 'p':
				// The default listing is already in a form `shopt` reads back.
			case 'o':
				return false, false, false, nil, fmt.Errorf(
					"-o: the `set -o` options are reached with `set`, not with shopt")
			default:
				return false, false, false, nil, fmt.Errorf("-%c: not an option; shopt takes -s -u -q -p", letter)
			}
		}
	}
	if set && unset {
		return false, false, false, nil, fmt.Errorf("-s and -u cannot both be given")
	}
	return set, unset, quiet, args[index:], nil
}

func shoptOptionNames() []string {
	names := make([]string, 0, len(shoptOptions))
	for _, option := range shoptOptions {
		names = append(names, option.name)
	}
	sort.Strings(names)
	return names
}

// listShoptOptions prints every option, or only the ones that are on when `-s` was
// given with no names -- which is bash's arrangement and how a script asks what is
// enabled.
func (r Runtime) listShoptOptions(onlyEnabled bool, all ...bool) int {
	for _, option := range shoptOptions {
		enabled := *option.field(r.options)
		if onlyEnabled && !enabled {
			continue
		}
		if len(all) == 0 && !onlyEnabled && !enabled {
			continue
		}
		r.printShoptOption(option)
	}
	return 0
}

func (r Runtime) printShoptOption(option shoptOption) {
	state := "off"
	if *option.field(r.options) {
		state = "on"
	}
	fmt.Fprintf(r.streams.Stdout, "%-12s\t%s\n", option.name, state)
}
