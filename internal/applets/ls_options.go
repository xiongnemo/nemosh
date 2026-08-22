package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// lsSortKey is which column the listing is ordered by. The options are mutually
// exclusive and the **last one given wins**, which is what GNU documents and
// what busybox-w32 does -- measured 2026-08-22: `ls -S -t` orders by time and
// `ls -t -S` by size.
type lsSortKey byte

const (
	lsSortByName lsSortKey = iota
	lsSortByTime
	lsSortBySize
)

type lsOptions struct {
	all bool
	// almostAll is -A: hidden entries, but not `.` and `..`. -a beats it in
	// either order, which is busybox's rule rather than GNU's -- measured, and
	// the simpler of the two to explain.
	almostAll bool
	long      bool
	human     bool
	color     colorWhen
	colored   bool
	// onePerLine is -1 and forceColumns is -C. Neither decides the layout on its own:
	// the destination does, and these override it. See ls_columns.go.
	onePerLine   bool
	forceColumns bool
	// width is -w, which implies columns because asking how wide they should be is
	// asking for them.
	width   int
	sortKey lsSortKey
	// reverse is -r, which reverses whatever order the sort key produced,
	// including the name tie-break.
	reverse bool
	// recursive is -R, and directoryItself is -d: the two opposite answers to
	// "what does a directory operand mean".
	recursive       bool
	directoryItself bool
	// classify is -F, appending one character that says what an entry is.
	classify bool
}

// lsShowsDotEntries reports whether `.` and `..` belong in the listing. -A asks
// for the hidden entries without them, which is the whole difference between the
// two options.
func (o lsOptions) lsShowsDotEntries() bool { return o.all }

// lsShowsHidden reports whether a name beginning with a dot is listed at all.
func (o lsOptions) lsShowsHidden() bool { return o.all || o.almostAll }

func lsArgs(args []string) (lsOptions, []string, error) {
	var options lsOptions
	index := 0
	for index < len(args) {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) <= 1 || arg[0] != '-' {
			break
		}
		// A long option is one word, so it is matched whole rather than letter
		// by letter -- `--color` used to be read as `-`, `-c`, `-o` and refused
		// as the bare `-` it started with.
		if strings.HasPrefix(arg, "--") {
			name, value, present := strings.Cut(arg[2:], "=")
			if name != "color" {
				return lsOptions{}, nil, fmt.Errorf("unsupported ls option: %s", arg)
			}
			when, err := parseColorWhen(value, present)
			if err != nil {
				return lsOptions{}, nil, err
			}
			options.color = when
			index++
			continue
		}
		// -w takes a number, so it cannot be read letter by letter with the rest.
		if strings.HasPrefix(arg, "-w") {
			value := arg[2:]
			if value == "" {
				if index+1 >= len(args) {
					return lsOptions{}, nil, fmt.Errorf("ls: -w requires a width")
				}
				index++
				value = args[index]
			}
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed <= 0 {
				return lsOptions{}, nil, fmt.Errorf("ls: invalid width: %s", value)
			}
			options.width = parsed
			index++
			continue
		}
		for _, flag := range arg[1:] {
			switch flag {
			case 'a':
				options.all = true
			case 'A':
				options.almostAll = true
			case 'l':
				options.long = true
			case 'h':
				options.human = true
			case '1':
				// -l wins over -1 whichever order the two are given, which is
				// what busybox does.
				options.onePerLine = true
			case 'C':
				options.forceColumns = true
			case 't':
				options.sortKey = lsSortByTime
			case 'S':
				options.sortKey = lsSortBySize
			case 'r':
				options.reverse = true
			case 'R':
				options.recursive = true
			case 'd':
				options.directoryItself = true
			case 'F':
				options.classify = true
			default:
				// Still refused by name: -i wants an inode number Windows does
				// not keep, -n a numeric owner this build does not resolve, and
				// -u/-c the access and change times, which NTFS records but
				// which no sort here reads yet. A script asking for one fails
				// rather than quietly getting something else.
				return lsOptions{}, nil, fmt.Errorf("unsupported ls option: -%c", flag)
			}
		}
		index++
	}
	return options, args[index:], nil
}
