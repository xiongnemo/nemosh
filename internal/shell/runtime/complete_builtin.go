package runtime

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/xiongnemo/nemosh/internal/shellquote"
)

// complete and compopt are bash's: what the line editor offers for a command's operands, said
// as compgen says it, and changed while a completion runs. busybox has no programmable
// completion, so bash is the reference throughout. They were "not found", so a completion
// script a tool prints -- `gh completion -s bash`, rustup's, npm's, git's -- stopped at its
// first `complete`.

// The names bash keeps the -D, -E and -I specifications under, which print as those options.
const (
	defaultCompletion = "_DefaultCmD_"
	emptyCompletion   = "_EmptycmD_"
	initialCompletion = "_InitialWorD_"
)

// completionTable is the specifications complete made, by command name, and the completion
// running, whose specification compopt with no name changes. Shared by a session's runtimes,
// as its history is: a subshell's `complete` is the session's too.
type completionTable struct {
	specs   map[string]compgenSpec
	running *runningCompletion
}

// runningCompletion is the specification a completion is using, a copy of the one found for
// name, which compopt changes for that completion alone.
type runningCompletion struct {
	name string
	spec *compgenSpec
}

func newCompletionTable() *completionTable {
	return &completionTable{specs: map[string]compgenSpec{}}
}

// completionBuiltin runs compgen, complete or compopt.
func (r Runtime) completionBuiltin(ctx context.Context, args []string) int {
	switch args[0] {
	case "complete":
		return r.complete(args[1:])
	case "compopt":
		return r.compopt(args[1:])
	}
	return r.compgen(ctx, args[1:], 0)
}

// completionTarget is the name -D, -E or -I stands for, and "" when none was given.
func completionTarget(flags string) string {
	switch {
	case strings.Contains(flags, "D"):
		return defaultCompletion
	case strings.Contains(flags, "E"):
		return emptyCompletion
	case strings.Contains(flags, "I"):
		return initialCompletion
	}
	return ""
}

// complete is complete_builtin: with -p, or with nothing to define, the specifications
// printed; with -r, removed; otherwise one made from the options for each name.
func (r Runtime) complete(args []string) int {
	options, index, ok := r.parseCompletionOptions("complete", args, "prDEI", "oAGWFCXPS")
	if !ok {
		return 2
	}
	names := args[index:]
	if target := completionTarget(options.flags); target != "" {
		names = []string{target}
	}
	switch {
	case options.has('p') || len(names) == 0 && !options.given:
		return r.printCompletions(names)
	case options.has('r'):
		return r.removeCompletions(names)
	case len(names) == 0:
		r.completionUsage("complete", "")
		return 2
	}
	for _, name := range names {
		r.completions.specs[name] = options.spec.clone()
	}
	return 0
}

// printCompletions prints the named specifications, or every one, as complete reads them back.
func (r Runtime) printCompletions(names []string) int {
	if len(names) == 0 {
		names = slices.Sorted(maps.Keys(r.completions.specs))
	}
	status := 0
	for _, name := range names {
		spec, found := r.completions.specs[name]
		if !found {
			fmt.Fprintf(r.streams.Stderr, "%scomplete: %s: no completion specification\n", r.diagnosticPrefix(), name)
			status = 1
			continue
		}
		fmt.Fprintln(r.streams.Stdout, "complete "+completionSpecText(spec)+completionNameText(name))
	}
	return status
}

// removeCompletions removes the named specifications, or every one.
func (r Runtime) removeCompletions(names []string) int {
	if len(names) == 0 {
		clear(r.completions.specs)
		return 0
	}
	status := 0
	for _, name := range names {
		if _, found := r.completions.specs[name]; !found {
			fmt.Fprintf(r.streams.Stderr, "%scomplete: %s: no completion specification\n", r.diagnosticPrefix(), name)
			status = 1
		}
		delete(r.completions.specs, name)
	}
	return status
}

// completionPrintOrder is bash's compacts table, the order complete prints the actions in:
// first the ones with a letter, then the rest with -A.
var completionPrintOrder = []string{
	"alias", "arrayvar", "binding", "builtin", "command", "directory", "disabled", "enabled",
	"export", "file", "function", "helptopic", "hostname", "group", "job", "keyword", "running",
	"service", "setopt", "shopt", "signal", "stopped", "user", "variable",
}

// completionSpecText is a specification's options as print_one_completion writes them, each
// followed by a blank: the -o names, the actions, then the values -- quoted, but for a
// function's name, which is quoted only when it needs to be.
func completionSpecText(spec compgenSpec) string {
	var text strings.Builder
	for _, name := range compgenOptionNames {
		if spec.options[name] {
			text.WriteString("-o " + name + " ")
		}
	}
	letters := map[string]byte{}
	for letter, action := range compgenLetters {
		letters[action] = letter
	}
	for _, lettered := range []bool{true, false} {
		for _, action := range completionPrintOrder {
			letter, hasLetter := letters[action]
			switch {
			case !slices.Contains(spec.actions, action) || hasLetter != lettered:
			case hasLetter:
				text.WriteString("-" + string(letter) + " ")
			default:
				text.WriteString("-A " + action + " ")
			}
		}
	}
	for _, value := range []struct{ flag, text string }{
		{"-G", spec.glob}, {"-W", spec.words}, {"-P", spec.prefix}, {"-S", spec.suffix},
		{"-X", spec.filter}, {"-C", spec.command},
	} {
		if value.text != "" || value.flag == "-W" && spec.hasWords {
			text.WriteString(value.flag + " " + shellquote.Single(value.text) + " ")
		}
	}
	if spec.function != "" {
		text.WriteString("-F " + quotedIfMeta(spec.function) + " ")
	}
	return text.String()
}

// completionNameText is the name a specification is for, as print_cmd_name writes it.
func completionNameText(name string) string {
	switch name {
	case defaultCompletion:
		return "-D"
	case emptyCompletion:
		return "-E"
	case initialCompletion:
		return "-I"
	}
	return quotedIfMeta(name)
}

// quotedIfMeta is text single-quoted when it holds what bash's sh_contains_shell_metas finds,
// and as it is otherwise.
func quotedIfMeta(text string) string {
	if text == "" || strings.ContainsAny(text, " \t\n'\"\\|&;()<>!{}*[?]^$`") ||
		text[0] == '#' || text[0] == '~' || strings.Contains(text, "=~") || strings.Contains(text, ":~") {
		return shellquote.Single(text)
	}
	return text
}
