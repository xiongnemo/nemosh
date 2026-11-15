package runtime

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// disown and enable are bash's, which busybox has not got; they were `not found`, 127, and a
// bashrc calls both. Each answer is bash 5.3's, measured.

// bashOnlyBuiltin runs disown or enable.
func (r Runtime) bashOnlyBuiltin(args []string) int {
	if args[0] == "enable" {
		return r.enable(args[1:])
	}
	return r.disown(args[1:])
}

const disownUsage = "disown: usage: disown [-h] [-ar] [jobspec ... | pid ...]"

// disown takes jobs out of the table, so `jobs`, `wait` and %N no longer see them: the
// current job, those named, every one with -a, or the running ones with -r. It does not end
// them. A job here outlives the shell whether or not it is disowned, since the shell sends no
// hangup when it exits -- which is all -h asks, so -h keeps the job and does nothing more.
// Closing the console window is another matter: Windows ends every process attached to it,
// a disowned job too.
func (r Runtime) disown(args []string) int {
	var all, running, keep bool
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		for _, letter := range args[0][1:] {
			switch letter {
			case 'a':
				all = true
			case 'r':
				running = true
			case 'h':
				keep = true
			default:
				fmt.Fprintf(r.streams.Stderr, "%sdisown: -%c: invalid option\n%s\n", r.diagnosticPrefix(), letter, disownUsage)
				return 2
			}
		}
		args = args[1:]
	}
	records, status := r.disownedJobs(args, all, running)
	if !keep {
		r.jobScope.forget(records)
	}
	return status
}

// disownedJobs are the records disown names, and the status to answer, 1 when one named no job.
func (r Runtime) disownedJobs(operands []string, all, running bool) ([]*jobRecord, int) {
	scope := r.jobScope
	if len(operands) == 0 && (all || running) {
		var records []*jobRecord
		for _, record := range scope.snapshot() {
			if !running || !record.finished() {
				records = append(records, record)
			}
		}
		return records, 0
	}
	if len(operands) == 0 {
		if id := scope.resolveJobSpec("%"); id != 0 {
			record, _ := scope.lookup(id)
			return []*jobRecord{record}, 0
		}
		fmt.Fprintf(r.streams.Stderr, "%sdisown: current: no such job\n", r.diagnosticPrefix())
		return nil, 1
	}
	var records []*jobRecord
	status := 0
	for _, operand := range operands {
		id := scope.resolveJobSpec(operand)
		if !strings.HasPrefix(operand, "%") {
			id = 0
			if pid, err := strconv.Atoi(operand); err == nil {
				id, _ = scope.lookupPID(pid)
			}
		}
		record, found := scope.lookup(id)
		if id == 0 || !found {
			fmt.Fprintf(r.streams.Stderr, "%sdisown: %s: no such job\n", r.diagnosticPrefix(), operand)
			status = 1
			continue
		}
		records = append(records, record)
	}
	return records, status
}

// finished reports whether the job has ended.
func (record *jobRecord) finished() bool {
	select {
	case <-record.done:
		return true
	default:
		return false
	}
}

const enableUsage = "enable: usage: enable [-a] [-dnps] [-f filename] [name ...]"

// enableRefusals are the options enable refuses, with why: a builtin here is part of the shell,
// so there is nothing to switch off and nothing to load.
var enableRefusals = map[rune]string{
	'n': "a builtin here cannot be switched off; a program of the same name runs when it is named by its path",
	'f': "this shell loads no builtins from a file",
	'd': "this shell loads no builtins from a file, so there are none to remove",
}

// enable lists the builtins as bash lists them, `enable NAME` a line each, or with -s only the
// special ones; and with names, says of each that is not one that it is not.
func (r Runtime) enable(args []string) int {
	special := false
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		for _, letter := range args[0][1:] {
			switch {
			case letter == 'a' || letter == 'p':
			case letter == 's':
				special = true
			case enableRefusals[letter] != "":
				fmt.Fprintf(r.streams.Stderr, "%senable: -%c: not implemented: %s\n", r.diagnosticPrefix(), letter, enableRefusals[letter])
				return 2
			default:
				fmt.Fprintf(r.streams.Stderr, "%senable: -%c: invalid option\n%s\n", r.diagnosticPrefix(), letter, enableUsage)
				return 2
			}
		}
		args = args[1:]
	}
	names := enabledBuiltins()
	if len(args) == 0 {
		for _, name := range names {
			if !special || isSpecialBuiltin(name) {
				fmt.Fprintf(r.streams.Stdout, "enable %s\n", name)
			}
		}
		return 0
	}
	status := 0
	for _, name := range args {
		if index := sort.SearchStrings(names, name); index == len(names) || names[index] != name {
			fmt.Fprintf(r.streams.Stderr, "%senable: %s: not a shell builtin\n", r.diagnosticPrefix(), name)
			status = 1
		}
	}
	return status
}

// enabledBuiltins are this shell's builtins in bash's sense, sorted: its own, and the applets
// busybox treats as builtins. bash-completion's helpers are left out, being no builtins of
// bash's, and so are the names refused as not implemented.
func enabledBuiltins() []string {
	var names []string
	for _, name := range builtinNames {
		if !strings.HasPrefix(name, "_") {
			names = append(names, name)
		}
	}
	for name := range ashBuiltinApplets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
