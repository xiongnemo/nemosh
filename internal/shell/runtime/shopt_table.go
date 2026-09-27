package runtime

// Every name bash 5.3's `shopt` has, and what each one is here. A script's first lines are
// often `shopt -s` of several names under `set -e`, and one name this shell did not know was
// status 1 there: the script ended before it began. So every name is known, and each says
// what it does. busybox has no shopt, so the names and their defaults are bash's.

// shoptKind is what one name does here.
type shoptKind uint8

const (
	// shoptActs changes what the shell does; field is its flag.
	shoptActs shoptKind = iota
	// shoptRecorded belongs to the interactive layer: history, completion, the terminal, the
	// prompt. It is remembered and reported and changes nothing, which is also what it changes
	// in a bash script. field remembers it.
	shoptRecorded
	// shoptFixed chooses between two behaviours, and this shell has only the one on says.
	// Asking for that one succeeds; asking for the other is refused, with why.
	shoptFixed
	// shoptStartup reports how the shell was started. As in bash, a request to change it
	// succeeds and changes nothing.
	shoptStartup
)

type shoptOption struct {
	name  string
	kind  shoptKind
	field func(*shellOptions) *bool
	// on is a fixed option's value, and any other's in a new shell.
	on bool
	// why says what a recorded option would do, or why a fixed one has the value it has.
	why string
}

func acts(name string, field func(*shellOptions) *bool) shoptOption {
	return shoptOption{name: name, kind: shoptActs, field: field}
}

// actsOn is an option that acts and is on in a new shell.
func actsOn(name string, field func(*shellOptions) *bool) shoptOption {
	return shoptOption{name: name, kind: shoptActs, field: field, on: true}
}

func recorded(name string, on bool, field func(*shellOptions) *bool, why string) shoptOption {
	return shoptOption{name: name, kind: shoptRecorded, field: field, on: on, why: why}
}

func fixed(name string, on bool, why string) shoptOption {
	return shoptOption{name: name, kind: shoptFixed, on: on, why: why}
}

const (
	noHistory    = "there is no history here to change"
	noCompletion = "completion here follows nemosh's own specs, which this does not reach"
	noCompat     = "there is one compatibility level, bash 5.3's where busybox is silent"
	notYet       = "not implemented yet"
)

// shoptOptions are in the order bash lists them, which is its Windows build's: that one adds
// completion_strip_exe.
var shoptOptions = []shoptOption{
	fixed("array_expand_once", false, "a subscript is expanded again when it is evaluated"),
	fixed("assoc_expand_once", false, "a subscript is expanded again when it is evaluated"),
	acts("autocd", func(o *shellOptions) *bool { return &o.autoCD }),
	fixed("bash_source_fullpath", false, "BASH_SOURCE holds a script's name as it was given"),
	fixed("cdable_vars", false, "cd takes its operand as a directory, never as a variable's name"),
	recorded("cdspell", false, func(o *shellOptions) *bool { return &o.cdSpell }, "cd corrects no spelling"),
	recorded("checkhash", false, func(o *shellOptions) *bool { return &o.checkHash },
		"a command is looked up afresh each time, so there is no remembered path to check"),
	recorded("checkjobs", false, func(o *shellOptions) *bool { return &o.checkJobs }, "exit does not list jobs first"),
	recorded("checkwinsize", true, func(o *shellOptions) *bool { return &o.checkWinSize },
		"the line editor asks the terminal its size itself"),
	recorded("cmdhist", true, func(o *shellOptions) *bool { return &o.cmdHist }, noHistory),
	fixed("compat31", false, noCompat),
	fixed("compat32", false, noCompat),
	fixed("compat40", false, noCompat),
	fixed("compat41", false, noCompat),
	fixed("compat42", false, noCompat),
	fixed("compat43", false, noCompat),
	fixed("compat44", false, noCompat),
	recorded("completion_strip_exe", false, func(o *shellOptions) *bool { return &o.completionStripExe }, noCompletion),
	recorded("complete_fullquote", true, func(o *shellOptions) *bool { return &o.completeFullQuote }, noCompletion),
	recorded("direxpand", false, func(o *shellOptions) *bool { return &o.dirExpand }, noCompletion),
	recorded("dirspell", false, func(o *shellOptions) *bool { return &o.dirSpell }, noCompletion),
	acts("dotglob", func(o *shellOptions) *bool { return &o.dotGlob }),
	fixed("execfail", false, "an exec that cannot run its command ends a script"),
	// On in a new shell, script or prompt, as busybox expands aliases in both.
	actsOn("expand_aliases", func(o *shellOptions) *bool { return &o.expandAliases }),
	fixed("extdebug", false, "there is no debugger support"),
	fixed("extglob", true, "the matcher knows ?() *() +() @() !() whether or not it is asked to"),
	fixed("extquote", false, "$'...' inside a double-quoted ${...} stays as written, as it does in busybox"),
	fixed("failglob", false, notYet),
	recorded("force_fignore", true, func(o *shellOptions) *bool { return &o.forceFignore }, noCompletion),
	fixed("globasciiranges", true, "a range in a bracket expression is by character code"),
	fixed("globskipdots", true, "a pattern never matches . or .."),
	acts("globstar", func(o *shellOptions) *bool { return &o.globStar }),
	fixed("gnu_errfmt", false, "an error message has one form"),
	recorded("histappend", false, func(o *shellOptions) *bool { return &o.histAppend }, noHistory),
	recorded("histreedit", false, func(o *shellOptions) *bool { return &o.histReedit }, noHistory),
	recorded("histverify", false, func(o *shellOptions) *bool { return &o.histVerify }, noHistory),
	recorded("hostcomplete", true, func(o *shellOptions) *bool { return &o.hostComplete }, noCompletion),
	recorded("huponexit", false, func(o *shellOptions) *bool { return &o.hupOnExit },
		"a job is not sent a hangup when the shell exits"),
	acts("inherit_errexit", func(o *shellOptions) *bool { return &o.inheritErrExit }),
	recorded("interactive_comments", true, func(o *shellOptions) *bool { return &o.interactiveComments },
		"a # at the start of a word always begins a comment"),
	acts("lastpipe", func(o *shellOptions) *bool { return &o.lastPipe }),
	recorded("lithist", false, func(o *shellOptions) *bool { return &o.litHist }, noHistory),
	acts("localvar_inherit", func(o *shellOptions) *bool { return &o.localVarInherit }),
	fixed("localvar_unset", true, "unsetting a caller's local leaves it unset until that function returns, as busybox has it"),
	{name: "login_shell", kind: shoptStartup},
	recorded("mailwarn", false, func(o *shellOptions) *bool { return &o.mailWarn }, "mail is not watched"),
	recorded("no_empty_cmd_completion", false, func(o *shellOptions) *bool { return &o.noEmptyCmdCompletion }, noCompletion),
	acts("nocaseglob", func(o *shellOptions) *bool { return &o.noCaseGlob }),
	acts("nocasematch", func(o *shellOptions) *bool { return &o.noCaseMatch }),
	fixed("noexpand_translation", false, "there is no message catalog to translate $\"...\" from"),
	acts("nullglob", func(o *shellOptions) *bool { return &o.nullGlob }),
	fixed("patsub_replacement", false, "& in a replacement stands for itself, as it does in busybox"),
	recorded("progcomp", true, func(o *shellOptions) *bool { return &o.progComp }, noCompletion),
	recorded("progcomp_alias", false, func(o *shellOptions) *bool { return &o.progCompAlias }, noCompletion),
	recorded("promptvars", true, func(o *shellOptions) *bool { return &o.promptVars },
		"the prompt's parameters are expanded either way"),
	{name: "restricted_shell", kind: shoptStartup},
	acts("shift_verbose", func(o *shellOptions) *bool { return &o.shiftVerbose }),
	actsOn("sourcepath", func(o *shellOptions) *bool { return &o.sourcePath }),
	fixed("varredir_close", false, "a {name} redirection's descriptor stays open after the command"),
	acts("xpg_echo", func(o *shellOptions) *bool { return &o.xpgEcho }),
}

func lookupShoptOption(name string) (shoptOption, bool) {
	for _, option := range shoptOptions {
		if option.name == name {
			return option, true
		}
	}
	return shoptOption{}, false
}

// newShellOptions is a new shell's options: all of them off, except the ones the table
// starts on.
func newShellOptions() *shellOptions {
	options := &shellOptions{}
	for _, option := range shoptOptions {
		if option.field != nil && option.on {
			*option.field(options) = true
		}
	}
	return options
}

// shoptValue is whether name is on here.
func (r Runtime) shoptValue(option shoptOption) bool {
	switch option.kind {
	case shoptActs, shoptRecorded:
		return *option.field(r.options)
	case shoptFixed:
		return option.on
	}
	return option.name == "login_shell" && r.options.login
}
