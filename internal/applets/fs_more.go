package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
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
	all       bool
	long      bool
	human     bool
	color     colorWhen
	colored   bool
	sizeWidth int
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
		for _, flag := range arg[1:] {
			switch flag {
			case 'a':
				options.all = true
			case 'l':
				options.long = true
			case 'h':
				options.human = true
			case '1':
				// One entry per line is what this ls always writes -- it never
				// lays out columns -- so -1 asks for the format already in use.
				// Accepting it is not pretending to do something: it is the
				// output that comes out either way, and busybox agrees on the
				// interaction, where -l wins whichever order the two are given.
				//
				// -C is the opposite case and stays refused, because columns are
				// the thing this cannot do.
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
		item := lsEntry{name: display, info: info}
		options.sizeWidth = lsEntrySizeWidth([]lsEntry{item}, options)
		return printLsEntry(stdout, item, options)
	}
	entries, err := os.ReadDir(target)
	if err != nil {
		return operandFailure(display, err)
	}
	items := make([]lsEntry, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !options.all && strings.HasPrefix(name, ".") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		items = append(items, lsEntry{name: name, info: info})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].name < items[j].name
	})
	options.sizeWidth = lsEntrySizeWidth(items, options)
	for _, item := range items {
		if err := printLsEntry(stdout, item, options); err != nil {
			return err
		}
	}
	return nil
}

type lsEntry struct {
	name string
	info os.FileInfo
}

func printLsEntry(stdout io.Writer, entry lsEntry, options lsOptions) error {
	name := paintLsName(entry.name, entry.info, options.colored)
	if options.long {
		_, err := fmt.Fprintf(stdout, "%s %*s %s\n", entry.info.Mode().String(), options.sizeWidth, lsSize(entry.info.Size(), options), name)
		return err
	}
	_, err := fmt.Fprintln(stdout, name)
	return err
}

func lsEntrySizeWidth(entries []lsEntry, options lsOptions) int {
	width := 0
	if options.human {
		width = 7
	}
	for _, entry := range entries {
		width = max(width, len(lsSize(entry.info.Size(), options)))
	}
	return width
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
