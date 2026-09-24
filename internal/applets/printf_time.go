package applets

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// printf's `%(format)T`, bash's: the operand is seconds since the epoch, rendered through
// strftime -- so `printf '%(%F %T)T' -1` is a timestamp without starting `date`, and it was
// `invalid conversion specification`. `-1` or nothing is now; `-2` is the shell's start in
// bash, which an applet cannot know, so it is now as well. An empty format is `%X`, as in
// bash. Width and flags apply to the rendered text: `%10(%Y)T` pads the year.

// printfTimeSpecification reads `%[flags][width][.precision](format)T` and reports the
// specification without its verb, the strftime format, and the bytes it occupies.
func printfTimeSpecification(rest string) (string, string, int, bool) {
	index := 1
	for index < len(rest) && strings.IndexByte("-+ #0", rest[index]) >= 0 {
		index++
	}
	for index < len(rest) && (rest[index] >= '0' && rest[index] <= '9' || rest[index] == '.') {
		index++
	}
	if index >= len(rest) || rest[index] != '(' {
		return "", "", 0, false
	}
	closing := strings.IndexByte(rest[index:], ')')
	if closing < 0 || index+closing+1 >= len(rest) || rest[index+closing+1] != 'T' {
		return "", "", 0, false
	}
	layout := rest[index+1 : index+closing]
	return rest[:index], layout, index + closing + 2, true
}

func renderPrintfTime(spec, layout, operand string) (string, error) {
	when := time.Now()
	var err error
	if trimmed := strings.TrimSpace(operand); trimmed != "" && trimmed != "-1" && trimmed != "-2" {
		seconds, parseErr := strconv.ParseInt(trimmed, 10, 64)
		if parseErr != nil {
			err = errPrintfNumber{operand: operand}
		}
		when = time.Unix(seconds, 0)
	}
	if layout == "" {
		layout = "%X"
	}
	rendered, _ := strftime(when, layout, false)
	return fmt.Sprintf(spec+"s", rendered), err
}
