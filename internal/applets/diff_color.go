package applets

import (
	"fmt"
	"strconv"
	"strings"
)

// diff's --color and its long options, which are GNU's: busybox's diff has neither.
//
// The colours are GNU's defaults (util.c's color_indicator, rs=0:hd=1:ad=32:de=31:ln=36): a
// header bold, an @@ line cyan, a line removed red and one added green, each closed with a reset
// before its newline. Under -y only a line on one side alone is coloured, and reset after the
// newline, as side.c does it.
const (
	diffColorHeader = "\x1b[1m"
	diffColorNumber = "\x1b[36m"
	diffColorReset  = "\x1b[0m"
)

func diffColorFor(removed bool) string {
	if removed {
		return "\x1b[31m"
	}
	return "\x1b[32m"
}

// diffLongOptions are GNU's names for the letters, and -y's three that have none: \x01 is
// --suppress-common-lines and \x02 --left-column. --unified takes no count here; -U does.
const diffLongOptions = "side-by-side=y width=W suppress-common-lines=\x01 left-column=\x02 " +
	"unified=u brief=q report-identical-files=s ignore-case=i ignore-all-space=w " +
	"ignore-space-change=b ignore-blank-lines=B new-file=N label=L initial-tab=T " +
	"expand-tabs=t text=a minimal=d recursive=r starting-file=S"

// diffColorOption takes --color[=WHEN], or --colour, out of args before the options are read,
// for its value is optional, which the option reader does not take. Plain --color is auto, as
// GNU's diff has it, where ls's is always. After -- nothing is an option.
func diffColorOption(args []string) (colorWhen, []string, error) {
	when := colorNever
	rest := make([]string, 0, len(args))
	for index, arg := range args {
		if arg == "--" {
			return when, append(rest, args[index:]...), nil
		}
		name, value, attached := strings.Cut(arg, "=")
		if name != "--color" && name != "--colour" {
			rest = append(rest, arg)
			continue
		}
		if !attached {
			when = colorAuto
			continue
		}
		var err error
		if when, err = parseColorWhen(value, true); err != nil {
			return when, nil, err
		}
	}
	return when, rest, nil
}

// diffWidth is -W's, the width -y lays its two columns out in, 130 when there is none.
func diffWidth(options appletOptions) (int, error) {
	if !options.has('W') {
		return 130, nil
	}
	width, err := strconv.Atoi(options.value('W'))
	if err != nil || width <= 0 {
		return 0, fmt.Errorf("invalid width '%s'", options.value('W'))
	}
	return width, nil
}
