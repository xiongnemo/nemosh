package runtime

import (
	"fmt"
	"strconv"

	"github.com/xiongnemo/nemosh/internal/proc"
)

// listKillSignals is `kill -l` and `trap -l`: the signals this shell can act on, one a line
// as busybox lists them, ` 1) HUP` -- not the whole POSIX set, since a name it would accept
// and then ignore would be worse than one it refuses.
//
// With operands each is translated instead, as both references do: a name to its number,
// and a number to its name. A number past 127 is read as an exit status, so `kill -l 143`
// is TERM, the signal that ended it. Busybox's rules throughout: 0 is EXIT, a number with no
// name here is given back as itself, and an unknown name is status 1. The operands were
// ignored, and the whole table printed for each.
func (r Runtime) listKillSignals(operands []string) int {
	if len(operands) == 0 {
		for _, signal := range proc.Signals() {
			fmt.Fprintf(r.streams.Stdout, "%2d) %s\n", signal.Number, signal.Name)
		}
		return 0
	}
	status := 0
	for _, operand := range operands {
		if number, err := strconv.Atoi(operand); err == nil {
			fmt.Fprintln(r.streams.Stdout, signalNameOf(number&0x7f))
			continue
		}
		number, err := proc.ParseSignal(operand)
		if err != nil {
			fmt.Fprintf(r.streams.Stderr, "%skill: unknown signal '%s'\n", r.diagnosticPrefix(), operand)
			status = 1
			continue
		}
		fmt.Fprintln(r.streams.Stdout, number)
	}
	return status
}

// signalNameOf is a signal's name, EXIT for 0, and the number itself for one with no name.
func signalNameOf(number int) string {
	if number == 0 {
		return "EXIT"
	}
	for _, signal := range proc.Signals() {
		if signal.Number == number {
			return signal.Name
		}
	}
	return strconv.Itoa(number)
}
