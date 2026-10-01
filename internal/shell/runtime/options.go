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
	// errTrace is `set -E`: the ERR trap is inherited by functions, subshells and
	// command substitutions, which it otherwise is not.
	errTrace bool
	// funcTrace is `set -T`: functions inherit the RETURN trap; see return_trap.go.
	funcTrace bool
	// noCaseGlob has no letter, like pipefail. It matters more on Windows
	// than elsewhere: NTFS is case-insensitive, so a pattern that fails only
	// because of case is surprising here in a way it is not on Unix.
	noCaseGlob bool
	// The three glob options `shopt` sets. No letters and no `set -o` names,
	// because bash keeps them on shopt; see builtin_shopt.go.
	globStar bool
	nullGlob bool
	dotGlob  bool
	// noCaseMatch is `shopt -s nocasematch`; see pattern_nocase.go.
	noCaseMatch bool
	// noHiddenGlob and noHidSysGlob are busybox-w32's: pathname expansion leaves out
	// a file with the Hidden attribute, or one that is both Hidden and System. See
	// glob_hidden.go.
	noHiddenGlob bool
	noHidSysGlob bool
	// ignoreEOF is `set -o ignoreeof`, -I: an end of input at a prompt is refused
	// rather than taken as `exit`. The session loops read it (IgnoresEOF).
	ignoreEOF bool
	// monitor is an accepted name that is refused when asked for; see inertShellOptions. vi is
	// the line editor's vi mode, which a session reads before each line it edits
	// (cmd/nemosh/lineedit_vi.go).
	monitor bool
	vi      bool
	// autoCD is `shopt -s autocd`: a bare directory name means `cd` to it. Off by
	// default, as it is in bash, because it changes what a mistyped command does.
	autoCD bool
	// inheritErrExit is `shopt -s inherit_errexit`: a command substitution keeps `set -e`,
	// which it otherwise does not; see commandSubstitutionScript.
	inheritErrExit bool
	// lastPipe is `shopt -s lastpipe`: a pipeline's last stage runs in the shell; see
	// lastpipe.go.
	lastPipe bool
	// expandAliases is `shopt -s expand_aliases`, on in a new shell: an alias is substituted
	// for the command name it matches; see alias_expand.go.
	expandAliases bool
	// shiftVerbose is `shopt -s shift_verbose`: a shift past the last parameter says so.
	shiftVerbose bool
	// xpgEcho is `shopt -s xpg_echo`: echo expands backslash escapes; see xpg_echo.go.
	xpgEcho bool
	// localVarInherit is `shopt -s localvar_inherit`: a new local starts as the caller's
	// variable was; see local_inherit.go.
	localVarInherit bool
	// sourcePath is `shopt -s sourcepath`, on in a new shell: `. name` looks on PATH; see
	// dot_source.go.
	sourcePath bool
	// failGlob is `shopt -s failglob`: a pattern that matches nothing is an error; see
	// failglob.go.
	failGlob bool
	// The shopt names of the interactive layer, which are remembered and reported and
	// change nothing; see shopt_table.go.
	cdSpell, checkHash, checkJobs, checkWinSize, cmdHist, completionStripExe     bool
	completeFullQuote, dirExpand, dirSpell, forceFignore, histAppend, histReedit bool
	histVerify, hostComplete, hupOnExit, interactiveComments, litHist, mailWarn  bool
	noEmptyCmdCompletion, progComp, progCompAlias, promptVars                    bool
	// bash's `set -o` names; see set_options_bash.go. braceExpand and hashAll are on in a
	// new shell, and histExpand, history and emacs in a session.
	braceExpand, hashAll, histExpand, emacs, history, noLog, posix, physical bool
	// login is whether the shell was started as a login shell, which `shopt login_shell`
	// reports.
	login bool
	// invocation is how the shell was started, as `$-` spells it after the options:
	// c for a command string, s for commands read from standard input, i for an
	// interactive session. Not options -- `set` cannot change them -- but kept here so
	// a subshell reports the same, as it does in both references.
	invocation string
}

type shellOptionSpec struct {
	letter byte
	name   string
	field  func(*shellOptions) *bool
	// bash marks a letter of bash's that busybox has not got, which `$-` leaves out; see
	// set_options_bash.go.
	bash bool
}

// shellOptionSpecs are busybox's options and then bash's.
var shellOptionSpecs = append(busyboxOptionSpecs, bashOptionSpecs...)

// A zero letter means the option has an `-o` name and no short form, which is
// how busybox carries pipefail.
var busyboxOptionSpecs = []shellOptionSpec{
	{'a', "allexport", func(o *shellOptions) *bool { return &o.allExport }, false},
	{'b', "notify", func(o *shellOptions) *bool { return &o.notify }, false},
	{'C', "noclobber", func(o *shellOptions) *bool { return &o.noClobber }, false},
	{'e', "errexit", func(o *shellOptions) *bool { return &o.errExit }, false},
	{'E', "errtrace", func(o *shellOptions) *bool { return &o.errTrace }, false},
	{'T', "functrace", func(o *shellOptions) *bool { return &o.funcTrace }, false},
	{'f', "noglob", func(o *shellOptions) *bool { return &o.noGlob }, false},
	{'I', "ignoreeof", func(o *shellOptions) *bool { return &o.ignoreEOF }, false},
	{'m', "monitor", func(o *shellOptions) *bool { return &o.monitor }, false},
	{'n', "noexec", func(o *shellOptions) *bool { return &o.noExec }, false},
	{'u', "nounset", func(o *shellOptions) *bool { return &o.noUnset }, false},
	{'v', "verbose", func(o *shellOptions) *bool { return &o.verbose }, false},
	{'x', "xtrace", func(o *shellOptions) *bool { return &o.xtrace }, false},
	{0, "pipefail", func(o *shellOptions) *bool { return &o.pipefail }, false},
	{0, "nocaseglob", func(o *shellOptions) *bool { return &o.noCaseGlob }, false},
	{0, "nohiddenglob", func(o *shellOptions) *bool { return &o.noHiddenGlob }, false},
	{0, "nohidsysglob", func(o *shellOptions) *bool { return &o.noHidSysGlob }, false},
	{0, "vi", func(o *shellOptions) *bool { return &o.vi }, false},
}

func (o *shellOptions) clone() *shellOptions {
	copied := *o
	return &copied
}

func shellOptionSpecByLetter(letter byte) (shellOptionSpec, bool) {
	for _, spec := range shellOptionSpecs {
		if spec.letter != 0 && spec.letter == letter {
			return spec, true
		}
	}
	return shellOptionSpec{}, false
}

func shellOptionSpecByName(name string) (shellOptionSpec, bool) {
	for _, spec := range shellOptionSpecs {
		if spec.name == name {
			return spec, true
		}
	}
	return shellOptionSpec{}, false
}

// busyboxLetters is busybox's option table, with how the shell was started among the options,
// as ash.c has it. `$-` spells it from the end, walking optlist from NOPTS-1 down: `set -eu`
// under -c is `uce`. T, which busybox has not got, sits after E, its pair in bash.
const busyboxLetters = "efIimnscxvCabuET"

// letters spells the enabled options the way `$-` reports them: the short letters that are on,
// and how the shell was started, in busyboxLetters' order backwards. An option with no short
// form has nothing to contribute.
func (o *shellOptions) letters() string {
	on := o.invocation
	for _, spec := range shellOptionSpecs {
		if spec.letter != 0 && !spec.bash && *spec.field(o) {
			on += string(spec.letter)
		}
	}
	var enabled strings.Builder
	for index := len(busyboxLetters) - 1; index >= 0; index-- {
		if strings.IndexByte(on, busyboxLetters[index]) >= 0 {
			enabled.WriteByte(busyboxLetters[index])
		}
	}
	return enabled.String()
}
