package applets

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// topArgs reads the options, a letter at a time and bundled as getopt has them: `top -bn1` is
// -b -n 1. They were matched a whole word at a time, so -bn1 was `unknown option -- b`.
//
// The letters are top's and htop's where they agree: -b batch, -n iterations, -d delay, -H
// threads, -t tree, -s sort, -f filter, -o columns, and procps-ng's -p and -u and -U, which
// choose processes by id and by account. -o is procps' too, where it names a field to sort by:
// an upper-case procps field, `-o %MEM` or `-o -PID`, is that sort, and a list of this top's own
// columns, which are lower-case, is still the layout.
func topArgs(args []string) (topOptions, error) {
	options := topOptions{iterations: 1, delay: topRefresh, sort: "cpu"}
	for index := 0; index < len(args); index++ {
		argument := args[index]
		switch {
		case strings.HasPrefix(argument, "--"):
			return options, unknownLongOption(argument)
		case len(argument) < 2 || argument[0] != '-':
			// Not an option, so not an option problem. top watches the whole machine and has
			// nothing to point at a file with, and saying that is more use than calling a path an
			// unsupported option.
			return options, fmt.Errorf("takes no operands: %s", argument)
		}
		for position := 1; position < len(argument); position++ {
			letter := argument[position]
			switch letter {
			case 'b':
				options.batch = true
				continue
			case 'H':
				options.threads = true
				continue
			case 't':
				options.tree = true
				continue
			case 'n', 'd', 's', 'o', 'f', 'p', 'u', 'U':
			default:
				return options, invalidOption(letter)
			}
			value := argument[position+1:]
			if value == "" {
				if index+1 >= len(args) {
					return options, fmt.Errorf("-%c requires a value", letter)
				}
				index++
				value = args[index]
			}
			if err := options.set(letter, value); err != nil || options.glossary {
				return options, err
			}
			break
		}
	}
	return options, nil
}

// set is what an option that takes a value does with it.
func (o *topOptions) set(letter byte, value string) error {
	switch letter {
	case 'n':
		count, err := strconv.Atoi(value)
		if err != nil || count < 1 {
			return fmt.Errorf("invalid iteration count: %s", value)
		}
		o.iterations = count
	case 'd':
		seconds, err := strconv.ParseFloat(value, 64)
		if err != nil || seconds <= 0 {
			return fmt.Errorf("invalid delay: %s", value)
		}
		o.delay = time.Duration(seconds * float64(time.Second))
	case 's', 'o':
		// `help` in place of a column name prints the glossary, which is htop's `--sort-key
		// help` and worth having for the same reason: it puts the terminology where it can be
		// read, grepped and pasted without opening the drawn form. That matters more here than on
		// Linux, because the drawn form needs a real console and several terminals on this
		// platform are not one.
		if value == "help" {
			o.glossary = true
			return nil
		}
		if letter == 's' {
			if _, ok := columnByKey(value); !ok {
				return fmt.Errorf("unknown sort column: %s", value)
			}
			o.sort = value
			return nil
		}
		return o.setLayoutOrSort(value)
	case 'f':
		o.filter = value
	case 'p':
		for _, word := range psList(value) {
			pid, err := strconv.Atoi(word)
			if err != nil || pid < 0 {
				return fmt.Errorf("invalid process ID list: %s", value)
			}
			o.pids = append(o.pids, pid)
		}
		if len(o.pids) == 0 {
			return fmt.Errorf("invalid process ID list: %s", value)
		}
	case 'u', 'U':
		names := psList(value)
		if len(names) == 0 {
			return errors.New("user name list syntax error")
		}
		o.users = append(o.users, names...)
	}
	return nil
}

// setLayoutOrSort is -o: procps' field to sort by, or this top's list of columns.
func (o *topOptions) setLayoutOrSort(value string) error {
	if key, direction, field, known := procpsSortField(value); field {
		if !known {
			return fmt.Errorf("unknown field: %s", strings.TrimLeft(value, "+-"))
		}
		o.sort, o.direction = key, direction
		return nil
	}
	o.columns = strings.Split(value, ",")
	if _, unknown := resolveColumns(o.columns); len(unknown) > 0 {
		return fmt.Errorf("unknown column: %s", strings.Join(unknown, ", "))
	}
	return nil
}

// procpsSortField reads procps' -o FIELD: a field's name after an optional `+`, high to low, or
// `-`, low to high. field is whether value is one -- a name procps has, or any upper-case word --
// and known whether this top has a column for it, whose key is answered.
func procpsSortField(value string) (string, int, bool, bool) {
	direction := 0
	switch {
	case strings.HasPrefix(value, "+"):
		direction, value = 1, value[1:]
	case strings.HasPrefix(value, "-"):
		direction, value = -1, value[1:]
	}
	switch value {
	case "%CPU":
		return "cpu", direction, true, true
	case "%MEM":
		return "mem", direction, true, true
	case "PID", "PPID", "USER":
		return strings.ToLower(value), direction, true, true
	case "PR":
		return "pri", direction, true, true
	case "S":
		return "state", direction, true, true
	case "RES":
		return "rss", direction, true, true
	case "TIME", "TIME+":
		return "time", direction, true, true
	case "COMMAND":
		return "command", direction, true, true
	case "nTH", "NTH":
		return "thr", direction, true, true
	}
	upper := value != "" && strings.ToUpper(value) == value && strings.ToLower(value) != value && !strings.Contains(value, ",")
	return "", 0, upper, false
}
