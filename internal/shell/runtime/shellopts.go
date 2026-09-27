package runtime

import (
	"slices"
	"strings"
)

// SHELLOPTS and BASHOPTS are bash's: the `set -o` names and the shopt names that are on,
// joined by colons and read afresh each time, so they are always current. Both are
// read-only. Exported, each carries its options into a child shell, which turns them on
// before anything else runs, as bash's does. busybox has neither.
var optionListNames = []string{"SHELLOPTS", "BASHOPTS"}

// optionList is the value of SHELLOPTS or BASHOPTS.
func (r Runtime) optionList(name string) string {
	var names []string
	if name == "SHELLOPTS" {
		for _, spec := range shellOptionSpecs {
			if *spec.field(r.options) {
				names = append(names, spec.name)
			}
		}
		slices.Sort(names)
	} else {
		for _, option := range shoptOptions {
			if r.shoptValue(option) {
				names = append(names, option.name)
			}
		}
	}
	return strings.Join(names, ":")
}

// adoptOptionLists is a new shell's start: SHELLOPTS and BASHOPTS become read-only, and an
// inherited one turns on the options it names -- a name this shell does not have is passed
// over, as bash passes it over -- and stays exported, with the value it has from then on.
func (r Runtime) adoptOptionLists() {
	for _, name := range optionListNames {
		if inherited, exported := r.env.LookupEnv(name); exported {
			for _, option := range strings.Split(inherited, ":") {
				if name == "SHELLOPTS" {
					_ = r.setOptionName(option, true)
				} else {
					_ = r.setShopt(option, true)
				}
			}
			delete(r.vars, name)
			r.env.Unset(name)
			r.markExported(name)
		}
		r.readonly[name] = struct{}{}
	}
}

// withOptionLists adds the current value of an exported SHELLOPTS or BASHOPTS to a child's
// environment, which has no fixed value of either to carry.
func (r Runtime) withOptionLists(environment []string) []string {
	for _, name := range optionListNames {
		if r.attributes[name].exported {
			environment = append(environment, name+"="+r.optionList(name))
		}
	}
	return environment
}
