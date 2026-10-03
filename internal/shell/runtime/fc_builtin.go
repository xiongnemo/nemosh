package runtime

import (
	"context"
	"fmt"
	"strings"
)

// fc is bash's, which is POSIX's with bash's defaults: busybox has none, and on Windows the
// name ran fc.exe, the file compare, which `fc.exe` still runs. It lists what history holds,
// hands some of it to an editor and runs what comes back, or runs one entry again.
//
// Every rule is bash's fc_builtin and fc_gethnum, the quirks among them: a FIRST that names
// the newest entry lists from the oldest, as bash's range check does.

const fcUsage = "fc: usage: fc [-e ename] [-lnr] [first] [last] or fc -s [pat=rep] [command]"

// fcRequest is fc's options.
type fcRequest struct {
	list, bare, reverse, again bool
	editor                     string
}

// fcFault is what a history specification can be besides an entry.
type fcFault int

const (
	fcFound fcFault = iota
	fcOutOfRange
	fcNotFound
)

// fcCommand is fc, a regular builtin whose assignments are its own -- `FCEDIT=vim fc` --
// run among the builtins that answer with a control transfer, because what it runs again
// may: an `exit` that `fc -s` runs leaves the shell.
func (r Runtime) fcCommand(ctx context.Context, args []string, assignments []assignment, operations []redirectOperation, savedStatus int) lineResult {
	runner := r
	if len(assignments) > 0 {
		temporary := r.withLocalAssignments(assignments)
		if temporary == nil {
			return lineResult{status: 1}
		}
		runner = *temporary
	}
	result := runner.withAppliedRedirectsFor(false, operations, func(redirected Runtime) lineResult {
		return redirected.fcResult(ctx, args, savedStatus)
	})
	if len(assignments) > 0 {
		r.mergeBuiltinMutations(runner)
	}
	return result
}

func (r Runtime) fcResult(ctx context.Context, args []string, savedStatus int) lineResult {
	request, operands, status := r.parseFcOptions(args)
	if status != 0 {
		return lineResult{status: status}
	}
	if request.again || request.editor == "-" {
		return r.fcAgain(ctx, operands, savedStatus)
	}
	entries := r.history.list()
	if len(entries) == 0 {
		return lineResult{}
	}
	first, last, ok := r.fcSpan(entries, operands, request.list)
	if !ok {
		return lineResult{status: 1}
	}
	if !request.list && r.history.lineWasAdded() {
		// "When not listing, the fc command that caused the editing shall not be entered
		// into the history list."
		newest := max(r.fcNewest(len(entries)), 0)
		r.history.takeBackLine()
		entries = r.history.list()
		first, last = fcAfterTakingBack(first, last, newest, len(entries))
	}
	if last < first {
		first, last, request.reverse = last, first, true
	}
	indices := fcIndices(first, last, request.reverse, len(entries))
	if request.list {
		r.fcList(entries, indices, request.bare)
		return lineResult{}
	}
	lines := make([]string, 0, len(indices))
	for _, index := range indices {
		lines = append(lines, entries[index])
	}
	return r.fcEdit(ctx, lines, request.editor, savedStatus)
}

// fcIndices is the entries from first to last, or from last to first, that the list has.
func fcIndices(first, last int, reverse bool, length int) []int {
	var indices []int
	for step := 0; step <= last-first; step++ {
		index := first + step
		if reverse {
			index = last - step
		}
		if index < length {
			indices = append(indices, index)
		}
	}
	return indices
}

// parseFcOptions reads the options as bash's internal_getopt reads ":e:lnrs", stopping at
// a word that is a number, so that `fc -l -3` lists from three back.
func (r Runtime) parseFcOptions(args []string) (fcRequest, []string, int) {
	var request fcRequest
	for len(args) > 0 && !fcNumberWord(args[0]) && len(args[0]) > 1 && args[0][0] == '-' {
		word := args[0]
		args = args[1:]
		if word == "--" {
			break
		}
		for index := 1; index < len(word); index++ {
			switch letter := word[index]; letter {
			case 'l':
				request.list = true
			case 'n':
				request.bare = true
			case 'r':
				request.reverse = true
			case 's':
				request.again = true
			case 'e':
				request.editor = word[index+1:]
				if request.editor == "" {
					if len(args) == 0 {
						return request, nil, r.fcMisuse("-e: option requires an argument")
					}
					request.editor, args = args[0], args[1:]
				}
				index = len(word)
			default:
				return request, nil, r.fcMisuse(fmt.Sprintf("-%c: invalid option", letter))
			}
		}
	}
	return request, args, 0
}

// fcNumberWord is bash's fc_number: a number, a minus sign before it allowed.
func fcNumberWord(word string) bool {
	_, ok := historyNumber(strings.TrimPrefix(word, "-"))
	return ok
}

func (r Runtime) fcMisuse(why string) int {
	fmt.Fprintf(r.streams.Stderr, "%sfc: %s\n%s\n", r.diagnosticPrefix(), why, fcUsage)
	return 2
}

// fcSpan is the entries FIRST and LAST name, counted from 0, as fc_builtin reads them: the
// newest sixteen to list when neither is given, the newest one to edit, and from FIRST to
// the newest, or FIRST alone, when only it is.
func (r Runtime) fcSpan(entries []string, operands []string, listing bool) (int, int, bool) {
	newest := r.fcNewest(len(entries))
	realLast := len(entries) - 1
	builtinLast := max(newest, 0)
	var first, last int
	var firstFault, lastFault fcFault
	switch {
	case len(operands) > 0:
		first, firstFault = fcEntryNumber(operands[0], entries, newest, realLast, listing, true)
		switch {
		case len(operands) > 1:
			last, lastFault = fcEntryNumber(operands[1], entries, newest, realLast, listing, false)
		case first == realLast && listing:
			last = realLast
		case listing:
			last = builtinLast
		default:
			last = first
		}
	case listing:
		last = builtinLast
		first = max(last-16+1, 0)
	default:
		first, last = builtinLast, builtinLast
	}
	switch {
	case firstFault == fcOutOfRange || lastFault == fcOutOfRange:
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"fc: history specification out of range")
		return 0, 0, false
	case firstFault == fcNotFound || lastFault == fcNotFound:
		fmt.Fprintln(r.streams.Stderr, r.diagnosticPrefix()+"fc: no command found")
		return 0, 0, false
	}
	// A specification out of range is no error, per POSIX.
	return max(first, 0), max(last, 0), true
}

// fcNewest is bash's last_hist: the newest entry fc deals in, the one before the fc now
// running when that was recorded. It can be -1, when the fc is all there is.
func (r Runtime) fcNewest(count int) int {
	newest := count
	if r.options.history {
		newest--
	}
	if r.history.lineWasAdded() {
		newest--
	}
	if newest == count {
		newest = count - 1
	}
	return newest
}

// fcAfterTakingBack keeps a span inside the list once the fc line is out of it, as
// fc_builtin does: newest is last_hist, and remaining how many entries are left.
func fcAfterTakingBack(first, last, newest, remaining int) (int, int) {
	if first == last && last == newest && newest >= remaining {
		last--
		first, newest = last, last
	}
	if newest >= remaining {
		newest--
	}
	if last >= newest {
		last = newest
	} else if first >= newest {
		first = newest
	}
	return max(first, 0), max(last, 0)
}

// fcEntryNumber is bash's fc_gethnum: a number from 1 as history numbers entries, one
// counted back from the newest when negative, or the newest entry that starts with the
// word.
func fcEntryNumber(spec string, entries []string, newest, realLast int, listing, first bool) (int, fcFault) {
	if newest < 0 {
		return -1, fcFound
	}
	digits, negative := strings.CutPrefix(spec, "-")
	if digits != "" && digits[0] >= '0' && digits[0] <= '9' {
		number := leadingNumber(digits)
		if negative {
			number = -number
		}
		switch {
		case number < 0:
			return max(number+newest+1, 0), fcFound
		case number == 0 && negative && listing:
			return realLast, fcFound
		case number == 0 && negative:
			return 0, fcOutOfRange
		case number == 0:
			return newest, fcFound
		}
		if number--; number >= newest {
			if first {
				return 0, fcFound
			}
			return newest, fcFound
		}
		return number, fcFound
	}
	for index := min(newest, len(entries)-1); index >= 0; index-- {
		if strings.HasPrefix(entries[index], spec) {
			return index, fcFound
		}
	}
	return 0, fcNotFound
}

// leadingNumber is atoi: the digits a word starts with, as a number, held below overflow.
func leadingNumber(digits string) int {
	number := 0
	for index := 0; index < len(digits) && digits[index] >= '0' && digits[index] <= '9'; index++ {
		number = min(number*10+int(digits[index]-'0'), 1<<30)
	}
	return number
}

// fcList is -l: each entry with its number and a tab, or with a tab alone under -n, and a
// blank after the tab where bash marks an entry it has changed, which this list never is.
func (r Runtime) fcList(entries []string, indices []int, bare bool) {
	separator := "\t "
	if r.options.posix {
		separator = "\t"
	}
	for _, index := range indices {
		if !bare {
			fmt.Fprint(r.streams.Stdout, index+1)
		}
		fmt.Fprintf(r.streams.Stdout, "%s%s\n", separator, entries[index])
	}
}
