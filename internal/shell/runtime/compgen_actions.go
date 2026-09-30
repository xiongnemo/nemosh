package runtime

import (
	"maps"
	"slices"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// compgenActionOrder is bash's order for compgen's actions (gen_action_completions): the ones
// the shell knows, then the ones on the filesystem, whatever order the options were given in.
var compgenActionOrder = []string{
	"alias", "arrayvar", "binding", "builtin", "disabled", "enabled", "export", "function",
	"helptopic", "job", "keyword", "running", "setopt", "shopt", "signal", "stopped", "variable",
	"command", "directory", "file", "group", "hostname", "service", "user",
}

// compgenKeywords are the words this shell reserves, in bash's order for them. time is bash's
// and not here, where time is busybox's applet.
var compgenKeywords = []string{
	"if", "then", "else", "elif", "fi", "case", "esac", "for", "select", "while", "until",
	"do", "done", "in", "function", "{", "}", "!", "[[", "]]", "coproc",
}

// compgenSignals are the signals trap and kill know, in their Linux numbers' order, then the
// shell's own conditions, as bash lists them.
var compgenSignals = []string{
	"EXIT", "SIGHUP", "SIGINT", "SIGQUIT", "SIGILL", "SIGTRAP", "SIGABRT", "SIGBUS", "SIGFPE",
	"SIGKILL", "SIGUSR1", "SIGSEGV", "SIGUSR2", "SIGPIPE", "SIGALRM", "SIGTERM", "SIGCHLD",
	"SIGCONT", "SIGSTOP", "SIGTSTP", "SIGTTIN", "SIGTTOU", "SIGURG", "SIGXCPU", "SIGXFSZ",
	"SIGVTALRM", "SIGPROF", "SIGPOLL", "SIGSYS", "DEBUG", "ERR", "RETURN",
}

// compgenActions is every action asked for, each once, in bash's order, as the names that
// begin with word.
func (r Runtime) compgenActions(actions []string, word string) []string {
	var matches []string
	for _, action := range compgenActionOrder {
		if slices.Contains(actions, action) {
			matches = append(matches, r.compgenAction(action, word)...)
		}
	}
	return matches
}

// compgenAction is one action's names. Those this shell has no source for -- bindings, groups,
// hosts, services, and jobs by name -- are none.
func (r Runtime) compgenAction(action, word string) []string {
	switch action {
	case "alias":
		return withPrefix(slices.Sorted(maps.Keys(r.aliases)), word)
	case "arrayvar":
		return withPrefix(r.arrayNames(), word)
	case "builtin", "enabled", "helptopic":
		// The shell's builtins, which `help` lists; an applet is a command, and -c has it.
		return withPrefix(slices.Sorted(slices.Values(builtinNames)), word)
	case "export":
		return withPrefix(r.exportedNames(), word)
	case "function":
		return withPrefix(r.functionNames(), word)
	case "keyword":
		return withPrefix(compgenKeywords, word)
	case "setopt":
		var names []string
		for _, spec := range shellOptionSpecs {
			names = append(names, spec.name)
		}
		slices.Sort(names)
		return withPrefix(slices.Compact(names), word)
	case "shopt":
		var names []string
		for _, option := range shoptOptions {
			names = append(names, option.name)
		}
		return withPrefix(names, word)
	case "signal":
		return withPrefix(compgenSignals, word)
	case "variable":
		names := append(slices.Collect(maps.Keys(r.vars)), r.arrayNames()...)
		slices.Sort(names)
		return withPrefix(slices.Compact(names), word)
	case "command":
		return r.compgenCommands(word)
	case "directory":
		return r.compgenPaths(word, true)
	case "file":
		return r.compgenPaths(word, false)
	case "user":
		if name := applets.CurrentUserName(); name != "" {
			return withPrefix([]string{name}, word)
		}
	}
	return nil
}

// compgenCommands is -c: what a command name may be, in bash's order -- aliases, reserved
// words, functions, builtins and applets, then what is on PATH -- each name once.
func (r Runtime) compgenCommands(word string) []string {
	var names []string
	names = append(names, slices.Sorted(maps.Keys(r.aliases))...)
	names = append(names, compgenKeywords...)
	names = append(names, r.functionNames()...)
	names = append(names, r.builtinAndAppletNames()...)
	names = append(names, r.pathCommandNames(word)...)
	seen := map[string]bool{}
	var kept []string
	for _, name := range withPrefix(names, word) {
		if !seen[name] {
			seen[name] = true
			kept = append(kept, name)
		}
	}
	return kept
}

func (r Runtime) arrayNames() []string {
	names := r.arrays.associativeNames()
	for name := range r.arrays.indexed {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// builtinAndAppletNames is what `builtin` runs: the shell's builtins and its applets.
func (r Runtime) builtinAndAppletNames() []string {
	names := append(slices.Clone(builtinNames), r.registry.Names()...)
	slices.Sort(names)
	return slices.Compact(names)
}

func (r Runtime) exportedNames() []string {
	var names []string
	for _, entry := range r.env.Environ() {
		if name, _, ok := strings.Cut(entry, "="); ok && name != "" {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func (r Runtime) functionNames() []string {
	var names []string
	for name := range r.functions {
		names = append(names, name.String())
	}
	slices.Sort(names)
	return names
}
