package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func newLsApplet() Applet {
	return simpleApplet{name: "ls", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, paths, err := lsArgs(args)
		if err != nil {
			return err
		}
		// `auto` is resolved against the stream actually being written to, not
		// at parse time: that is what makes `alias ls='ls --color=auto'` safe to
		// pipe, colouring a terminal and staying plain into grep.
		options.colored = colorEnabled(options.color, stdout)
		if len(paths) == 0 {
			paths = []string{"."}
		}
		view := ProcessViewFromContext(ctx)
		for _, target := range paths {
			// A device is described from the table rather than resolved to a host
			// path it has not got. `ls -l /dev/null` answered "is not a host path"
			// before this, where busybox prints a character device.
			if info, err := statDeviceOperand(view, target); err != nil {
				return err
			} else if info != nil {
				if err := printLsEntry(stdout, lsEntry{name: target, info: info, path: target}, options); err != nil {
					return err
				}
				continue
			}
			native, err := resolveHostPath(view, target)
			if err != nil {
				return err
			}
			if err := listPath(stdout, native, target, options); err != nil {
				return err
			}
		}
		return nil
	}}
}

type lsOptions struct {
	all     bool
	long    bool
	human   bool
	color   colorWhen
	colored bool
	// onePerLine is -1 and forceColumns is -C. Neither decides the layout on its own:
	// the destination does, and these override it. See ls_columns.go.
	onePerLine   bool
	forceColumns bool
	// width is -w, which implies columns because asking how wide they should be is
	// asking for them.
	width int
}

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
			default:
				return lsOptions{}, nil, fmt.Errorf("unsupported ls option: -%c", flag)
			}
		}
		index++
	}
	return options, args[index:], nil
}

func listPath(stdout io.Writer, target, display string, options lsOptions) error {
	info, err := os.Stat(target)
	if err != nil {
		return operandFailure(display, err)
	}
	if !info.IsDir() {
		return printLsEntry(stdout, lsEntry{name: display, info: info, path: target}, options)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return operandFailure(display, err)
	}
	items := make([]lsEntry, 0, len(entries)+2)
	if options.all {
		items = append(items, lsDotEntries(target)...)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !options.all && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		items = append(items, lsEntry{name: name, info: info, path: filepath.Join(target, name)})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	if !options.long {
		return writeLsNames(stdout, items, options, lsWantsColumns(options, stdout))
	}
	// `total N` heads a directory listing and not a list of file operands, which is what
	// both references do.
	//
	// The blocks are the *apparent* size rounded up, not du's allocated size, and that is
	// measured rather than chosen: busybox says `total 4` for a directory holding only `.`
	// and `..`, which is the 4096 it reports for `..` divided by the block size -- while
	// busybox's own `du` says 0 for the same directory. The two answers come from different
	// rules in the reference itself, and this follows each where it is used.
	total := int64(0)
	for _, item := range items {
		total += (item.info.Size() + duBlock - 1) / duBlock
	}
	if _, err := fmt.Fprintf(stdout, "total %d\n", total); err != nil {
		return err
	}
	for _, item := range items {
		if err := printLsEntry(stdout, item, options); err != nil {
			return err
		}
	}
	return nil
}

// lsDotEntries are `.` and `..`, which -a lists and which this omitted entirely.
//
// os.ReadDir does not report them -- it is a directory *contents* call -- so they are added
// back. Both references list them, and `ls -la` without them cannot show a directory's own mode.
func lsDotEntries(target string) []lsEntry {
	var entries []lsEntry
	for name, path := range map[string]string{".": target, "..": filepath.Dir(target)} {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		entries = append(entries, lsEntry{name: name, info: info, path: path})
	}
	return entries
}

type lsEntry struct {
	name string
	info os.FileInfo
	// path is where the file really is. The long form has to ask the filesystem two more
	// questions -- the link count and the owner -- and neither can be asked of a name.
	path string
}

func printLsEntry(stdout io.Writer, entry lsEntry, options lsOptions) error {
	name := paintLsName(entry.name, entry.info, options.colored)
	if options.long {
		// The whole line is busybox-w32's layout; see ls_long.go.
		return writeLongEntry(stdout, entry.path, name, entry.info,
			lsSize(entry.info.Size(), options), lsSizeField(options))
	}
	_, err := fmt.Fprintln(stdout, name)
	return err
}

// lsSizeField is how wide the size column is. Ten for a number and eight for a human size,
// both measured from busybox -- `1.5K` sits two columns further left there than a count of
// bytes would.
func lsSizeField(options lsOptions) int {
	if options.human {
		return 8
	}
	return 10
}

func lsSize(size int64, options lsOptions) string {
	if !options.human || size < 1024 {
		return fmt.Sprintf("%d", size)
	}
	units := []string{"K", "M", "G", "T"}
	value := float64(size)
	unit := ""
	for _, candidate := range units {
		value /= 1024
		unit = candidate
		if value < 1024 {
			break
		}
	}
	if value >= 10 || value == float64(int64(value)) {
		return fmt.Sprintf("%.0f%s", value, unit)
	}
	return fmt.Sprintf("%.1f%s", value, unit)
}
