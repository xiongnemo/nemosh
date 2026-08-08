package runtime

import "strings"

// shellOptions holds the `set` flags. The letters and their `-o` long names are
// the ones busybox ash carries in optletters_optnames (shell/ash.c:481), minus
// the ones that need a job-control terminal.
//
// Every accepted option is stored even where nothing reads it yet, so `$-` and
// `set -o` report the shell's real state rather than a subset. Which ones are
// still inert is recorded in docs/design/v0-readiness.md rather than hidden.
type shellOptions struct {
	allExport bool
	notify    bool
	noClobber bool
	errExit   bool
	noGlob    bool
	noExec    bool
	noUnset   bool
	verbose   bool
	xtrace    bool
	pipefail  bool
	// noCaseGlob has no letter, like pipefail. It matters more on Windows
	// than elsewhere: NTFS is case-insensitive, so a pattern that fails only
	// because of case is surprising here in a way it is not on Unix.
	noCaseGlob bool
}

type shellOptionSpec struct {
	letter byte
	name   string
	field  func(*shellOptions) *bool
}

// A zero letter means the option has an `-o` name and no short form, which is
// how busybox carries pipefail.
var shellOptionSpecs = []shellOptionSpec{
	{'a', "allexport", func(o *shellOptions) *bool { return &o.allExport }},
	{'b', "notify", func(o *shellOptions) *bool { return &o.notify }},
	{'C', "noclobber", func(o *shellOptions) *bool { return &o.noClobber }},
	{'e', "errexit", func(o *shellOptions) *bool { return &o.errExit }},
	{'f', "noglob", func(o *shellOptions) *bool { return &o.noGlob }},
	{'n', "noexec", func(o *shellOptions) *bool { return &o.noExec }},
	{'u', "nounset", func(o *shellOptions) *bool { return &o.noUnset }},
	{'v', "verbose", func(o *shellOptions) *bool { return &o.verbose }},
	{'x', "xtrace", func(o *shellOptions) *bool { return &o.xtrace }},
	{0, "pipefail", func(o *shellOptions) *bool { return &o.pipefail }},
	{0, "nocaseglob", func(o *shellOptions) *bool { return &o.noCaseGlob }},
}

func (o *shellOptions) clone() *shellOptions {
	copied := *o
	return &copied
}

func (o *shellOptions) byLetter(letter byte) (*bool, bool) {
	for _, spec := range shellOptionSpecs {
		if spec.letter != 0 && spec.letter == letter {
			return spec.field(o), true
		}
	}
	return nil, false
}

func shellOptionSpecByName(name string) (shellOptionSpec, bool) {
	for _, spec := range shellOptionSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return shellOptionSpec{}, false
}

// letters spells the enabled options the way `$-` reports them: the short
// letters that are on, in table order. An option with no short form has nothing
// to contribute.
func (o *shellOptions) letters() string {
	var enabled strings.Builder
	for _, spec := range shellOptionSpecs {
		if spec.letter != 0 && *spec.field(o) {
			enabled.WriteByte(spec.letter)
		}
	}
	return enabled.String()
}
