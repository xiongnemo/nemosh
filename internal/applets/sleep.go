package applets

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

const maxSleepDuration = time.Duration(1<<63 - 1)

var errInvalidSleepDuration = errors.New("invalid sleep duration")

var _ = newSleepApplet

func newSleepApplet() Applet {
	return sleepApplet{}
}

type sleepApplet struct{}

func (sleepApplet) Name() string {
	return "sleep"
}

func (sleepApplet) Run(ctx context.Context, args []string, _ io.Reader, _ io.Writer, stderr io.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	operands := args
	if len(operands) > 0 && operands[0] == "--" {
		operands = operands[1:]
	}
	if len(operands) == 0 {
		return writeSleepDiagnostic(stderr, "sleep: missing operand")
	}

	total := time.Duration(0)
	for _, operand := range operands {
		duration, err := parseSleepDuration(operand)
		if err != nil {
			return writeSleepDiagnostic(stderr, fmt.Sprintf("sleep: invalid duration %q", operand))
		}
		if duration > maxSleepDuration-total {
			return writeSleepDiagnostic(stderr, fmt.Sprintf("sleep: invalid duration %q", operand))
		}
		total += duration
	}

	timer := time.NewTimer(total)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseSleepDuration(input string) (time.Duration, error) {
	if input == "" {
		return 0, errInvalidSleepDuration
	}

	unit := time.Second
	number := input
	switch input[len(input)-1] {
	case 's':
		number = input[:len(input)-1]
	case 'm':
		unit = time.Minute
		number = input[:len(input)-1]
	case 'h':
		unit = time.Hour
		number = input[:len(input)-1]
	case 'd':
		unit = 24 * time.Hour
		number = input[:len(input)-1]
	}

	sawDigit := false
	sawDot := false
	for _, char := range number {
		switch {
		case char >= '0' && char <= '9':
			sawDigit = true
		case char == '.' && !sawDot:
			sawDot = true
		default:
			return 0, errInvalidSleepDuration
		}
	}
	if !sawDigit {
		return 0, errInvalidSleepDuration
	}

	value, err := strconv.ParseFloat(number, 64)
	if err != nil || value >= float64(maxSleepDuration)/float64(unit) {
		return 0, errInvalidSleepDuration
	}
	duration := time.Duration(value * float64(unit))
	if duration < 0 {
		return 0, errInvalidSleepDuration
	}
	return duration, nil
}

func writeSleepDiagnostic(stderr io.Writer, message string) error {
	if _, err := fmt.Fprintln(stderr, message); err != nil {
		return err
	}
	return ErrExitFalse
}
