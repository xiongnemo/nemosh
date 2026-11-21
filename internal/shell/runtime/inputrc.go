package runtime

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// An inputrc, read as readline reads one: lines of its init syntax (bind_line.go), `set keymap
// NAME` choosing the keymap the lines after it bind in, `$if`, `$else` and `$endif`, nested,
// and `$include FILE`. `$if` tests mode=emacs or vi, term=NAME -- TERM, or its part before the
// first dash -- the application, Bash, `version` against readline 8.2, bash 5.2's, and
// otherwise a readline variable, NAME=VALUE. bind -f reads a file so, and a session in bash mode
// reads its inputrc so before its startup files, as readline has read it by the time a bashrc's
// bind runs.

const inputrcVersion = 8.2

// ReadInputrc is a session's inputrc in bash mode: $INPUTRC, or else ~/.inputrc, or else
// /etc/inputrc. One that is not there is nothing to say; one that cannot be read is said.
func (r Runtime) ReadInputrc() {
	for _, path := range r.inputrcPaths() {
		data, err := os.ReadFile(r.resolvePath(path))
		if err == nil {
			r.bindInputrc(r.defaultKeymap(), "", string(data), 0)
			return
		}
		if !os.IsNotExist(err) {
			fmt.Fprintf(r.streams.Stderr, "%s%s: %s\n", r.diagnosticPrefix(), path, applets.CauseText(err))
			return
		}
	}
}

func (r Runtime) inputrcPaths() []string {
	if path, set := r.LookupEnv("INPUTRC"); set && path != "" {
		return []string{path}
	}
	var paths []string
	if home := r.vars["HOME"]; home != "" {
		paths = append(paths, strings.TrimRight(home, `/\`)+"/.inputrc")
	}
	return append(paths, "/etc/inputrc")
}

// inputrcLevel is one $if: whether its lines would be read at all, and whether its test chose
// the lines before $else or after it.
type inputrcLevel struct{ outer, on bool }

// bindInputrc reads text as an inputrc in keymap, and answers 1 when a line was refused.
func (r Runtime) bindInputrc(keymap, prefix, text string, depth int) int {
	status := 0
	var levels []inputrcLevel
	active := func() bool { return len(levels) == 0 || levels[len(levels)-1].outer && levels[len(levels)-1].on }
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "$if"):
			levels = append(levels, inputrcLevel{outer: active(), on: r.inputrcTest(strings.TrimSpace(line[3:]))})
		case line == "$else" && len(levels) > 0:
			levels[len(levels)-1].on = !levels[len(levels)-1].on
		case line == "$endif" && len(levels) > 0:
			levels = levels[:len(levels)-1]
		case !active():
		case strings.HasPrefix(line, "$include"):
			status = max(status, r.includeInputrc(keymap, prefix, strings.TrimSpace(line[len("$include"):]), depth))
		case strings.HasPrefix(line, "set keymap "):
			if named, namedPrefix, ok := keymapNamed(strings.TrimSpace(line[len("set keymap "):])); ok {
				keymap, prefix = named, namedPrefix
			}
		default:
			if err := r.bindLine(keymap, prefix, line); err != nil {
				status = r.bindFailed(err)
			}
		}
	}
	return status
}

// includeInputrc is `$include FILE`, ~ its home, no deeper than readline would go.
func (r Runtime) includeInputrc(keymap, prefix, path string, depth int) int {
	if depth >= 10 {
		return 0
	}
	if rest, home := strings.CutPrefix(path, "~/"); home {
		path = strings.TrimRight(r.vars["HOME"], `/\`) + "/" + rest
	}
	data, err := os.ReadFile(r.resolvePath(path))
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "%sbind: %s: cannot read: %s\n", r.diagnosticPrefix(), path, applets.CauseText(err))
		return 1
	}
	return r.bindInputrc(keymap, prefix, string(data), depth+1)
}

// inputrcTest is an $if's condition.
func (r Runtime) inputrcTest(condition string) bool {
	switch {
	case strings.HasPrefix(condition, "mode="):
		mode := "emacs"
		if r.options.vi {
			mode = "vi"
		}
		return strings.TrimSpace(condition[len("mode="):]) == mode
	case strings.HasPrefix(condition, "term="):
		want, term := strings.TrimSpace(condition[len("term="):]), r.vars["TERM"]
		short, _, _ := strings.Cut(term, "-")
		return term != "" && (term == want || short == want)
	case strings.HasPrefix(condition, "version"):
		return inputrcVersionTest(strings.TrimSpace(condition[len("version"):]))
	case strings.EqualFold(condition, "bash"):
		return true
	}
	name, value, found := strings.Cut(condition, "=")
	return found && r.ReadlineVariable(strings.TrimSpace(name)) == strings.TrimSpace(strings.TrimPrefix(value, "="))
}

// inputrcVersionTest is `version OP NUMBER` against readline 8.2.
func inputrcVersionTest(test string) bool {
	for _, operator := range [...]string{">=", "<=", "==", "!=", ">", "<", "="} {
		rest, found := strings.CutPrefix(test, operator)
		if !found {
			continue
		}
		number, err := strconv.ParseFloat(strings.TrimSpace(rest), 64)
		if err != nil {
			return false
		}
		switch operator {
		case ">=":
			return inputrcVersion >= number
		case "<=":
			return inputrcVersion <= number
		case ">":
			return inputrcVersion > number
		case "<":
			return inputrcVersion < number
		case "!=":
			return inputrcVersion != number
		}
		return inputrcVersion == number
	}
	return false
}
