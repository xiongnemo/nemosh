package applets

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/xiongnemo/nemosh/internal/textgrid"
)

// The lists ps's options take -- columns, sort keys, pids, accounts and names -- and what each
// adds to the options.

// valued is an option that takes a list: of columns, pids, users or command names.
func (o *psOptions) valued(letter byte, value string) error {
	switch letter {
	case 'o':
		columns, err := parsePsFormat(value)
		if err != nil {
			return err
		}
		if err := o.setStyle(psCustom); err != nil {
			return err
		}
		o.columns = append(o.columns, columns...)
	case 'p', 'q':
		pids, err := parsePsPids(value)
		if err != nil {
			return err
		}
		o.pids = append(o.pids, pids...)
	case 'u', 'U':
		names := psList(value)
		if len(names) == 0 {
			return errors.New("user name list syntax error")
		}
		o.users = append(o.users, names...)
	case 'C':
		names := psList(value)
		if len(names) == 0 {
			return errors.New("command name list syntax error")
		}
		o.commands = append(o.commands, names...)
	}
	return nil
}

// setStyle allows one kind of format, as procps does: -f, u and -o do not mix.
func (o *psOptions) setStyle(style psStyle) error {
	if o.style != psDefault && o.style != style {
		return errors.New("conflicting format options")
	}
	o.style = style
	return nil
}

// addSort is --sort's and k's keys, between commas, each a column's name after an optional `+`,
// or a `-` for descending.
func (o *psOptions) addSort(spec string) error {
	for _, key := range strings.Split(spec, ",") {
		descending := strings.HasPrefix(key, "-")
		field, ok := lookupPsField(strings.TrimLeft(key, "+-"))
		if !ok {
			return fmt.Errorf("unknown sort specifier %q", key)
		}
		o.sort = append(o.sort, psSortKey{field: field, descending: descending})
	}
	return nil
}

// parsePsFormat is a format list, procps-ng's way: items between commas or blanks, each a
// specifier with an optional `:WIDTH` and `=HEADER`, the header ending where the item does.
// busybox's last header takes the rest of the argument instead; `-o pid= -o comm=` reads the same
// under either.
func parsePsFormat(text string) ([]psColumn, error) {
	// A separator at the end is let go, as procps lets it go; one anywhere else leaves an
	// empty item, which is an improper list.
	text = strings.TrimRight(text, " ,\t\n")
	if text == "" {
		return nil, errors.New("improper format list")
	}
	var columns []psColumn
	for _, item := range splitPsItems(text) {
		if item == "" {
			return nil, errors.New("improper format list")
		}
		spec, header, renamed := strings.Cut(item, "=")
		name, size, sized := strings.Cut(spec, ":")
		field, ok := lookupPsField(name)
		if !ok {
			return nil, fmt.Errorf("unknown user-defined format specifier %q", name)
		}
		column := psColumn{field: field, header: field.header, width: field.width}
		if sized {
			width, err := strconv.Atoi(size)
			if err != nil || width <= 0 {
				return nil, fmt.Errorf("invalid width in format specifier %q", spec)
			}
			column.width = width
		}
		if renamed {
			column.header = header
		}
		column.width = max(column.width, textgrid.Cells(column.header))
		columns = append(columns, column)
	}
	return columns, nil
}

// splitPsItems cuts at every separator, so two together leave an empty item between them.
func splitPsItems(text string) []string {
	var items []string
	for {
		end := strings.IndexFunc(text, isPsSeparator)
		if end < 0 {
			return append(items, text)
		}
		items = append(items, text[:end])
		text = text[end+1:]
	}
}

func isPsSeparator(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }

// psList is a list of names, between commas or blanks.
func psList(text string) []string { return strings.FieldsFunc(text, isPsSeparator) }

// parsePsPids is a list of process ids, which are decimal digits and nothing else.
func parsePsPids(text string) ([]int, error) {
	var pids []int
	for _, word := range psList(text) {
		pid, err := strconv.Atoi(word)
		if err != nil || strings.Trim(word, "0123456789") != "" {
			return nil, errors.New("process ID list syntax error")
		}
		pids = append(pids, pid)
	}
	if len(pids) == 0 {
		return nil, errors.New("process ID list syntax error")
	}
	return pids, nil
}
