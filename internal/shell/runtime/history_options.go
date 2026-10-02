package runtime

import (
	"fmt"
	"strconv"
	"strings"
)

// `history`'s options are bash's: busybox's history prints the list and takes nothing
// else, so it is bash's to answer what -d, -s and the file options do. Only -c was
// taken; the rest were "unsupported", status 2.
//
// The order is bash's history_builtin: no more than one of -anrw; -c clears and, with
// nothing after it, is done; then -s, -p, -d, the listing, and the file options.

const historyUsage = "history: usage: history [-c] [-d offset] [n] or history -anrw [filename] or history -ps arg [arg...]"

// historyRequest is the letters given, and -d's position.
type historyRequest struct {
	letters  string
	position string
}

func (request historyRequest) has(letter byte) bool {
	return strings.IndexByte(request.letters, letter) >= 0
}

// historyBuiltin prints the numbered list, or changes it, or moves it to or from a file.
func (r Runtime) historyBuiltin(args []string) int {
	request, operands, status := r.parseHistoryOptions(args)
	if status != 0 {
		return status
	}
	files := 0
	for _, letter := range []byte("anrw") {
		if request.has(letter) {
			files++
		}
	}
	if files > 1 {
		fmt.Fprintln(r.streams.Stderr, "history: cannot use more than one of -anrw")
		return 1
	}
	if request.has('c') {
		r.history.clear()
		if len(operands) == 0 {
			return 0
		}
	}
	switch {
	case request.has('s'):
		// The words are one entry, put where the `history -s` that wrote them was.
		if len(operands) > 0 {
			r.history.takeBackLine()
			r.addHistoryLine(strings.Join(operands, " "), false)
		}
		return 0
	case request.has('p'):
		return r.printHistoryExpansions(operands)
	case request.has('d'):
		return r.deleteHistory(request.position)
	case files == 0 && !request.has('c'):
		return r.listHistory(operands)
	}
	return r.historyFileRequest(request, operands)
}

// parseHistoryOptions reads the options as bash's internal_getopt reads "acd:npsrw": in
// one word or several, -d's position the rest of its word or the next.
func (r Runtime) parseHistoryOptions(args []string) (historyRequest, []string, int) {
	var request historyRequest
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		word := args[0]
		args = args[1:]
		if word == "--" {
			break
		}
		for index := 1; index < len(word); index++ {
			letter := word[index]
			if strings.IndexByte("acnprsw", letter) >= 0 {
				request.letters += string(letter)
				continue
			}
			if letter != 'd' {
				return request, nil, r.historyMisuse(fmt.Sprintf("-%c: invalid option", letter))
			}
			request.letters += "d"
			request.position = word[index+1:]
			if request.position == "" {
				if len(args) == 0 {
					return request, nil, r.historyMisuse("-d: option requires an argument")
				}
				request.position, args = args[0], args[1:]
			}
			break
		}
	}
	return request, args, 0
}

// historyMisuse says what was wrong and how history is used, status 2, as bash's does.
func (r Runtime) historyMisuse(why string) int {
	fmt.Fprintf(r.streams.Stderr, "history: %s\n%s\n", why, historyUsage)
	return 2
}

// listHistory is `history [n]`: the newest n entries, or every one.
func (r Runtime) listHistory(operands []string) int {
	count := -1
	if len(operands) > 0 {
		number, ok := historyNumber(operands[0])
		if !ok {
			fmt.Fprintf(r.streams.Stderr, "history: %s: numeric argument required\n", operands[0])
			return 2
		}
		if len(operands) > 1 {
			fmt.Fprintln(r.streams.Stderr, "history: too many arguments")
			return 2
		}
		count = int(min(max(number, -number), maxHistoryEntries))
	}
	r.printHistory(count)
	return 0
}

// historyNumber reads a number as bash's legal_number does: blanks about it allowed, and
// nothing else.
func historyNumber(text string) (int64, bool) {
	number, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	return number, err == nil
}

// deleteHistory is -d: one position as `history` numbers it, or one counted back from the
// end, or a range of either, START-END.
func (r Runtime) deleteHistory(position string) int {
	length := int64(len(r.history.list()))
	sign := 0
	if strings.HasPrefix(position, "-") {
		sign = 1
	}
	if cut := strings.IndexByte(position[sign:], '-'); cut >= 0 {
		first, last := position[:sign+cut], position[sign+cut+1:]
		start, startOK := historyNumber(first)
		end, endOK := historyNumber(last)
		if !startOK || !endOK {
			return r.historyOutOfRange(position)
		}
		if start = historyIndex(first, start, length); start < 0 || start >= length {
			return r.historyOutOfRange(first)
		}
		if end = historyIndex(last, end, length); end < 0 || end >= length {
			return r.historyOutOfRange(last)
		}
		return historyStatus(r.history.remove(int(start), int(end)))
	}
	offset, ok := historyNumber(position)
	if !ok {
		fmt.Fprintf(r.streams.Stderr, "history: %s: invalid number\n", position)
		return 1
	}
	index := offset - 1
	if strings.HasPrefix(position, "-") && offset < 0 {
		index = length + offset
	}
	if index < 0 || index >= length {
		return r.historyOutOfRange(position)
	}
	return historyStatus(r.history.remove(int(index), int(index)))
}

// historyIndex is a range end as bash reads one: counted back from the end when it is
// written negative, and from 1 when it is positive; 0 is the first entry all the same.
func historyIndex(text string, number, length int64) int64 {
	switch {
	case strings.HasPrefix(text, "-") && number < 0:
		return number + length
	case number > 0:
		return number - 1
	}
	return number
}

func (r Runtime) historyOutOfRange(position string) int {
	fmt.Fprintf(r.streams.Stderr, "history: %s: history position out of range\n", position)
	return 1
}

func historyStatus(done bool) int {
	if done {
		return 0
	}
	return 1
}
