package applets

import (
	"math"
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

// lsTimeKey is which of a file's times -l shows and -t orders by: -c the change time, which on
// Windows is when the file was made, as busybox-w32's st_ctime is, and -u when it was last
// read. Of the two the last given wins, and without -l either orders by itself.
type lsTimeKey byte

const (
	lsModifyTime lsTimeKey = iota
	lsChangeTime
	lsAccessTime
)

// lsFollow is which symbolic links are followed: those named on the command line, unless the
// listing asks about entries themselves; only those, -H; or every one, -L.
type lsFollow byte

const (
	lsFollowOperands lsFollow = iota
	lsFollowNamed
	lsFollowAll
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
	// onePerLine is -1, forceColumns -C and across -x, columns filled a row at a time. None
	// decides the layout on its own: the destination does, and these override it. Of -C, -x
	// and -l the last given wins, and of -1 and -C or -x; see ls_columns.go.
	onePerLine, forceColumns, across bool
	width                            int
	sortKey                          lsSortKey
	// version and extension are -v and -X: they order what the sort key leaves tied, and the
	// name what they leave tied, as busybox's sortcmp does.
	version, extension bool
	// dirsFirst is --group-directories-first.
	dirsFirst bool
	// reverse is -r, which reverses whatever order the sort key produced,
	// including the name tie-break.
	reverse bool
	// recursive is -R, and directoryItself is -d: the two opposite answers to
	// "what does a directory operand mean".
	recursive       bool
	directoryItself bool
	// classify is -F, appending one character that says what an entry is, and slash -p,
	// which appends a directory's `/` and nothing else.
	classify, slash bool
	// groupOnly is -g, the long form without the owner, and numeric -n, ids for names.
	groupOnly, numeric bool
	// inode and blocks are -i and -s, each a column before the name.
	inode, blocks bool
	// quote is -Q, a name in double quotes with C escapes, and printable -q, a `?` for what
	// cannot be shown; see ls_names.go.
	quote, printable bool
	timeKey          lsTimeKey
	fullTime         bool
	follow           lsFollow
	// umask is the shell's, which the mode column is made up through; see lsModeString.
	umask uint32
}

// lsShowsDotEntries reports whether `.` and `..` belong in the listing. -A asks
// for the hidden entries without them, which is the whole difference between the
// two options.
func (o lsOptions) lsShowsDotEntries() bool { return o.all }

// lsShowsHidden reports whether a name beginning with a dot is listed at all.
func (o lsOptions) lsShowsHidden() bool { return o.all || o.almostAll }

// lsArgs splits the options from the operands, which with permute they may follow, as getopt
// lets them: `ls dir -l`.
func lsArgs(args []string, permute bool) (lsOptions, []string, error) {
	var options lsOptions
	var operands []string
	index := 0
	for index < len(args) {
		arg := args[index]
		if arg == "--" {
			index++
			break
		}
		if len(arg) <= 1 || arg[0] != '-' {
			if !permute {
				break
			}
			operands = append(operands, arg)
			index++
			continue
		}
		used, err := options.word(args, index)
		if err != nil {
			return lsOptions{}, nil, err
		}
		index += used
	}
	// Without -l, -c and -u order by the time they choose, as busybox's do.
	if !options.long && options.timeKey != lsModifyTime && options.sortKey == lsSortByName {
		options.sortKey = lsSortByTime
	}
	return options, append(operands, args[index:]...), nil
}

// word reads one word of options, answering how many words it took: two when -w or -T takes
// the next.
//
// A long option is one word, so it is matched whole rather than letter by letter -- `--color`
// used to be read as `-`, `-c`, `-o` and refused as the bare `-` it started with.
func (o *lsOptions) word(args []string, index int) (int, error) {
	arg := args[index]
	if strings.HasPrefix(arg, "--") {
		name, value, present := strings.Cut(arg[2:], "=")
		switch {
		case name == "color":
			when, err := parseColorWhen(value, present)
			o.color = when
			return 1, err
		case name == "full-time" && !present:
			o.fullTime = true
			o.setLayout('l')
			return 1, nil
		case name == "group-directories-first" && !present:
			o.dirsFirst = true
			return 1, nil
		}
		return 0, unknownLongOption(arg)
	}
	for position := 1; position < len(arg); position++ {
		letter := arg[position]
		if letter != 'w' && letter != 'T' {
			if err := o.letter(letter); err != nil {
				return 0, err
			}
			continue
		}
		// -w and -T take the rest of the word or the next one. -T is a tab size, taken and
		// ignored as busybox takes it.
		value, used := arg[position+1:], 1
		if value == "" {
			if index+1 >= len(args) {
				return 0, missingOptionArgument(string(letter))
			}
			value, used = args[index+1], 2
		}
		width, err := positiveNumber(value)
		if err != nil {
			return 0, err
		}
		if letter == 'w' {
			// -w 0 is no limit, as busybox's is.
			o.width = width
			if width == 0 {
				o.width = math.MaxInt32
			}
		}
		return used, nil
	}
	return 1, nil
}

func (o *lsOptions) letter(letter byte) error {
	switch letter {
	case 'a':
		o.all = true
	case 'A':
		o.almostAll = true
	case 'l', 'C', 'x', '1':
		o.setLayout(letter)
	case 'g', 'n':
		// Each is the long form, as busybox's getopt makes them imply -l.
		o.groupOnly, o.numeric = o.groupOnly || letter == 'g', o.numeric || letter == 'n'
		o.setLayout('l')
	case 'h':
		o.human = true
	case 't':
		o.sortKey = lsSortByTime
	case 'S':
		o.sortKey = lsSortBySize
	case 'v':
		o.version = true
	case 'X':
		o.extension = true
	case 'c':
		o.timeKey = lsChangeTime
	case 'u':
		o.timeKey = lsAccessTime
	case 'r':
		o.reverse = true
	case 'R':
		o.recursive = true
	case 'd':
		o.directoryItself = true
	case 'F':
		o.classify = true
	case 'p':
		o.slash = true
	case 'i':
		o.inode = true
	case 's':
		o.blocks = true
	case 'Q':
		o.quote = true
	case 'q':
		o.printable = true
	case 'L':
		o.follow = lsFollowAll
	case 'H':
		o.follow = lsFollowNamed
	case 'k':
		// -k is the size in kilobytes, which -s already is; busybox takes it and ignores it.
	default:
		// Still refused by name: -Z is an SELinux context no Windows file has, and -m and
		// -o are options busybox does not have either.
		return invalidOption(letter)
	}
	return nil
}

// setLayout is busybox's getopt pairs for the layout: -C, -x and -l each undo the other two,
// and -1 undoes -C and -x as they undo it, so of each pair the last given wins -- `ls -l -C`
// is columns. -l beats -1 in either order, which is what busybox does too.
func (o *lsOptions) setLayout(letter byte) {
	switch letter {
	case 'l':
		o.long, o.forceColumns, o.across = true, false, false
	case 'C':
		o.forceColumns, o.across, o.long, o.onePerLine = true, false, false, false
	case 'x':
		o.across, o.forceColumns, o.long, o.onePerLine = true, false, false, false
	case '1':
		o.onePerLine, o.forceColumns, o.across = true, false, false
	}
}
