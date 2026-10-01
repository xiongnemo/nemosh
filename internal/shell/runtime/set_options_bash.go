package runtime

import "fmt"

// bash's `set -o` names that busybox has not got. `set +H` and `set -o posix` begin a good
// many scripts, and both were "illegal option": status 2, and under `set -e` the end of the
// script. Now each is one of these:
//
//   - acting: braceexpand (-B) turns brace expansion off and on; physical (-P) makes cd and
//     pwd follow links, see directory_options.go; histexpand (-H) and history are the
//     prompt's `!` expansion and its history, on in a session and off in a script, as in
//     bash; emacs turns busybox's vi off, the line editor's other mode (set_builtin.go);
//   - recorded: remembered and reported, changing nothing -- hashall (-h), since a command is
//     looked up afresh each time; nolog, which bash ignores too; posix, since the behaviour
//     here is busybox ash's, a POSIX shell's, either way; interactive-comments, shared with
//     shopt's name;
//   - fixed: igncr is always on, as a script's carriage returns are dropped here as in
//     busybox-w32; keyword (-k), onecmd (-t) and privileged (-p) are always off. The value
//     there is is accepted, and the other refused with the reason.
//
// Their letters are left out of `$-`: bash spells B and h there, and busybox, which decides
// what `$-` says, has neither.
var bashOptionSpecs = []shellOptionSpec{
	{letter: 'B', name: "braceexpand", field: func(o *shellOptions) *bool { return &o.braceExpand }, bash: true},
	{letter: 'h', name: "hashall", field: func(o *shellOptions) *bool { return &o.hashAll }, bash: true},
	{letter: 'H', name: "histexpand", field: func(o *shellOptions) *bool { return &o.histExpand }, bash: true},
	{name: "emacs", field: func(o *shellOptions) *bool { return &o.emacs }},
	{name: "history", field: func(o *shellOptions) *bool { return &o.history }},
	{name: "igncr", field: alwaysOn},
	{name: "interactive-comments", field: func(o *shellOptions) *bool { return &o.interactiveComments }},
	{letter: 'k', name: "keyword", field: alwaysOff, bash: true},
	{name: "nolog", field: func(o *shellOptions) *bool { return &o.noLog }},
	{letter: 't', name: "onecmd", field: alwaysOff, bash: true},
	{letter: 'P', name: "physical", field: func(o *shellOptions) *bool { return &o.physical }, bash: true},
	{name: "posix", field: func(o *shellOptions) *bool { return &o.posix }},
	{letter: 'p', name: "privileged", field: alwaysOff, bash: true},
}

// fixedShellOptions are the names whose behaviour this shell has one of, and why.
var fixedShellOptions = map[string]string{
	"igncr":      "a carriage return before a newline is dropped, as busybox-w32 drops it",
	"keyword":    "an assignment after the command name is an argument",
	"onecmd":     "a script runs to its end",
	"privileged": "there is no set-user-ID on Windows to keep",
}

// alwaysOff and alwaysOn are the flags of fixed options: a fresh value each time, so reading
// one answers the fixed value and writing one changes nothing.
func alwaysOff(*shellOptions) *bool { return new(bool) }

func alwaysOn(*shellOptions) *bool {
	on := true
	return &on
}

// fixedOptionRefusal refuses the value a fixed option does not have.
func fixedOptionRefusal(spec shellOptionSpec, enable bool) error {
	why, fixed := fixedShellOptions[spec.name]
	if !fixed || enable == *spec.field(&shellOptions{}) {
		return nil
	}
	state := "off"
	if !enable {
		state = "on"
	}
	return fmt.Errorf("%co %s: always %s here: %s", optionSign(enable), spec.name, state, why)
}

// braceWords is brace expansion, unless `set +B` turned it off.
func (r Runtime) braceWords(item word) []word {
	if !r.options.braceExpand {
		return []word{item}
	}
	return expandBraceWord(item)
}
