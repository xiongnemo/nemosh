package applets

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// pingArguments reads iputils' options -- -c -i -W -w -s -t, -q -n -O -4 -6 -- and the one HOST.
// A second word, or one beginning with /, is likelier Windows' ping typed from habit than a
// mistake, and says where that one is.
func pingArguments(ctx context.Context, args []string) (pingSettings, string, error) {
	settings := pingSettings{size: 56, interval: time.Second}
	options, operands, err := parseAppletOptions(ctx, args, "qnO46", "ciWwst")
	if err != nil {
		return settings, "", err
	}
	switch {
	case len(operands) == 0:
		return settings, "", ExitStatusMessage(2, errors.New("usage error: Destination address required"))
	case len(operands) > 1 || strings.HasPrefix(operands[0], "/"):
		return settings, "", ExitStatusMessage(2, errPingWindowsHabit)
	}
	settings.quiet, settings.numeric, settings.overdue = options.has('q'), options.has('n'), options.has('O')
	if options.has('4') {
		settings.family = 4
	} else if options.has('6') {
		settings.family = 6
	}
	integer := func(letter byte, low, high int, into *int) error {
		if !options.has(letter) {
			return nil
		}
		value, err := strconv.Atoi(options.value(letter))
		if err != nil || value < low || value > high {
			return ExitStatusMessage(2, fmt.Errorf("invalid argument: '%s': out of range: %d <= value <= %d", options.value(letter), low, high))
		}
		*into = value
		return nil
	}
	seconds := func(letter byte, into *time.Duration) error {
		if !options.has(letter) {
			return nil
		}
		value, err := strconv.ParseFloat(options.value(letter), 64)
		if err != nil || value < 0 || value > 1e6 {
			return ExitStatusMessage(2, fmt.Errorf("invalid argument: '%s'", options.value(letter)))
		}
		*into = time.Duration(value * float64(time.Second))
		return nil
	}
	for _, read := range []error{
		integer('c', 1, 1<<31-1, &settings.count), integer('s', 0, 65500, &settings.size), integer('t', 1, 255, &settings.ttl),
		seconds('i', &settings.interval), seconds('W', &settings.wait), seconds('w', &settings.deadline),
	} {
		if read != nil {
			return settings, "", read
		}
	}
	return settings, operands[0], nil
}
